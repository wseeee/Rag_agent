package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"SuperBizAgent/internal/ai/embedder"
	cfg "SuperBizAgent/internal/config"
	"SuperBizAgent/internal/model"
	"SuperBizAgent/pkg/client"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

var (
	defaultManager *MemoryManager
	managerOnce    sync.Once
)

// MemoryManager 三层分层记忆中枢管理器
type MemoryManager struct {
	redisStore  *RedisStore
	milvusStore *MilvusStore
	summarizer  *Summarizer
	config      MemoryConfig
}

// GetDefaultMemoryManager 获取全局单例分层记忆管理器
func GetDefaultMemoryManager() *MemoryManager {
	managerOnce.Do(func() {
		ctx := context.Background()
		var err error
		defaultManager, err = NewDefaultMemoryManager(ctx)
		if err != nil {
			g.Log().Errorf(ctx, "[MemoryManager] 初始化分层记忆单例失败: %v", err)
		}
	})
	return defaultManager
}

// NewDefaultMemoryManager 从全局配置自动组装三层存储
func NewDefaultMemoryManager(ctx context.Context) (*MemoryManager, error) {
	shortTermWindow := cfg.C.Memory.ShortTermWindow
	if shortTermWindow <= 0 {
		shortTermWindow = 10
	}
	summaryTriggerTurns := cfg.C.Memory.SummaryTriggerTurns
	if summaryTriggerTurns <= 0 {
		summaryTriggerTurns = 10
	}
	ttlDays := cfg.C.Memory.TTLDays
	if ttlDays <= 0 {
		ttlDays = 7
	}
	longTermThreshold := cfg.C.Memory.LongTermThreshold
	if longTermThreshold <= 0 {
		longTermThreshold = 0.70
	}
	topK := cfg.C.Memory.TopK
	if topK <= 0 {
		topK = 5
	}
	milvusCollection := cfg.C.Memory.MilvusCollection
	if milvusCollection == "" {
		milvusCollection = "user_memory"
	}

	// 1. Redis 配置与初始化 (动态计算 2 倍轮次作为消息条数窗口)
	rdb := client.GetRedisClient()
	redisStore := NewRedisStore(rdb, time.Duration(ttlDays)*24*time.Hour, shortTermWindow*2)

	// 2. Milvus 配置与初始化
	var milvusStore *MilvusStore
	milvusCli, err := client.NewMilvusClient(ctx)
	if err == nil {
		eb, ebErr := embedder.DoubaoEmbedding(ctx)
		if ebErr == nil {
			milvusStore, err = NewMilvusStore(ctx, milvusCli, eb, milvusCollection, longTermThreshold, topK)
			if err != nil {
				g.Log().Warningf(ctx, "[MemoryManager] Milvus 记忆库初始化告警: %v", err)
			}
		} else {
			g.Log().Warningf(ctx, "[MemoryManager] 阿里 DashScope Embedder 初始化告警: %v", ebErr)
		}
	} else {
		g.Log().Warningf(ctx, "[MemoryManager] Milvus 客户端初始化告警: %v", err)
	}

	// 3. Summarizer 初始化
	summarizer, err := NewSummarizer(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[MemoryManager] 对话摘要器初始化告警: %v", err)
	}

	memCfg := MemoryConfig{
		ShortTermWindow:     shortTermWindow,
		SummaryTriggerTurns: summaryTriggerTurns,
		TTL:                 time.Duration(ttlDays) * 24 * time.Hour,
		ScoreThreshold:      longTermThreshold,
		TopK:                topK,
		MilvusCollection:    milvusCollection,
	}

	return &MemoryManager{
		redisStore:  redisStore,
		milvusStore: milvusStore,
		summarizer:  summarizer,
		config:      memCfg,
	}, nil
}

// GetHistory 供上层 Controller 单行调用的上下文获取入口
func (m *MemoryManager) GetHistory(ctx context.Context, sessionID, userID, query string) []*schema.Message {
	if m == nil {
		return nil
	}
	_, msgs, _ := m.GetAssembledContext(ctx, sessionID, userID, query)
	return msgs
}

// GetAssembledContext 在提问前组装三层上下文：阶段摘要 + 高置信度长期记忆 + 近轮对话
func (m *MemoryManager) GetAssembledContext(
	ctx context.Context,
	sessionID string,
	userID string,
	query string,
) (*AssembledContext, []*schema.Message, error) {
	if userID == "" {
		userID = "default_user"
	}

	assembled := &AssembledContext{}

	// 1. 读取短期记忆：优先从 Redis 读取
	var recentMsgs []*model.Message
	if m.redisStore != nil {
		msgs, err := m.redisStore.GetRecentMessages(ctx, sessionID, m.config.ShortTermWindow*2)
		if err == nil && len(msgs) > 0 {
			recentMsgs = msgs
		}
	}

	// 若 Redis 未命中或已过期，从 MySQL (model.MessageModel) 懒加载并回填 Redis
	if len(recentMsgs) == 0 {
		msgs, err := model.MessageModel.GetRecentMessages(ctx, sessionID, m.config.ShortTermWindow*2)
		if err == nil && len(msgs) > 0 {
			recentMsgs = msgs
			if m.redisStore != nil {
				_ = m.redisStore.PushMessages(ctx, sessionID, msgs)
			}
		}
	}
	assembled.RecentMessages = recentMsgs

	// 2. 读取阶段累进摘要：优先 Redis，次选 MySQL 懒加载
	var summary string
	if m.redisStore != nil {
		s, err := m.redisStore.GetSummary(ctx, sessionID)
		if err == nil {
			summary = s
		}
	}
	if summary == "" {
		conv, err := model.ConversationModel.GetOrCreate(ctx, sessionID, userID, "新对话")
		if err == nil && conv != nil {
			summary = conv.Summary
			if summary != "" && m.redisStore != nil {
				_ = m.redisStore.SetSummary(ctx, sessionID, summary)
			}
		}
	}
	assembled.Summary = summary

	// 3. 检索 Milvus 长期情节记忆 (严格阈值截断 >= 0.70)
	var longTerms []*MemoryItem
	if m.milvusStore != nil && query != "" {
		lts, err := m.milvusStore.SearchLongTerm(ctx, query, userID, m.config.TopK, m.config.ScoreThreshold)
		if err == nil {
			longTerms = lts
		} else {
			g.Log().Warningf(ctx, "[MemoryManager] 检索长期记忆异常: %v", err)
		}
	}
	assembled.LongTermMemories = longTerms

	// 4. 将三层记忆格式化为 Eino Prompt 可用的 []*schema.Message 结构
	einoMessages := make([]*schema.Message, 0)

	// ① 前序阶段背景摘要（作为 System 补充）
	if summary != "" {
		einoMessages = append(einoMessages, schema.SystemMessage(
			fmt.Sprintf("【前序会话背景摘要（供参考）】：\n%s", summary),
		))
	}

	// ② 高相关度长期情节记忆（作为 System 补充）
	if len(longTerms) > 0 {
		var memBuilder strings.Builder
		memBuilder.WriteString(fmt.Sprintf("【召回的高置信度历史排障与经验记忆（相关度 >= %.2f）】：\n", m.config.ScoreThreshold))
		for idx, lt := range longTerms {
			memBuilder.WriteString(fmt.Sprintf("%d. %s\n", idx+1, lt.Content))
		}
		einoMessages = append(einoMessages, schema.SystemMessage(memBuilder.String()))
	}

	// ③ 近轮原始问答上下文
	for _, msg := range recentMsgs {
		switch msg.Role {
		case "user":
			einoMessages = append(einoMessages, schema.UserMessage(msg.Content))
		case "assistant":
			einoMessages = append(einoMessages, schema.AssistantMessage(msg.Content, nil))
		case "system":
			einoMessages = append(einoMessages, schema.SystemMessage(msg.Content))
		}
	}

	return assembled, einoMessages, nil
}

// RecordInteractionAsync 后台异步非阻塞持久化三层记忆（MySQL流水 + Redis热缓存 + 增量摘要计算 + Milvus长期记忆）
func (m *MemoryManager) RecordInteractionAsync(
	sessionID string,
	userID string,
	userContent string,
	assistantContent string,
	promptTokens int,
	completionTokens int,
	metadata string,
) {
	if sessionID == "" || (userContent == "" && assistantContent == "") {
		return
	}
	if userID == "" {
		userID = "default_user"
	}

	// 启动后台 Goroutine，绝不阻塞前端流式输出
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		now := time.Now()
		userMsg := &model.Message{
			ConversationID: sessionID,
			Role:           "user",
			Content:        userContent,
			PromptTokens:   promptTokens,
			CreatedAt:      now,
		}
		asstMsg := &model.Message{
			ConversationID:   sessionID,
			Role:             "assistant",
			Content:          assistantContent,
			CompletionTokens: completionTokens,
			CreatedAt:        now.Add(time.Millisecond),
		}
		if metadata != "" {
			asstMsg.Metadata = &metadata
		}

		// 1. 持久化至 MySQL 事实真理源 (直调 model.MessageModel 并获取当前累计总消息数)
		totalCount, err := model.MessageModel.SaveInteraction(ctx, sessionID, userID, userContent, assistantContent, promptTokens, completionTokens, metadata)
		if err != nil {
			g.Log().Errorf(ctx, "[MemoryManager] 异步保存 MySQL 消息流水失败: %v", err)
		}

		// 2. 写入 Redis 短期工作记忆队列
		if m.redisStore != nil {
			if err := m.redisStore.PushMessages(ctx, sessionID, []*model.Message{userMsg, asstMsg}); err != nil {
				g.Log().Warningf(ctx, "[MemoryManager] 异步写入 Redis 缓存失败: %v", err)
			}
		}

		// 3. 检查是否触发增量摘要提炼 (按每 SummaryTriggerTurns 轮 = triggerCount 条消息周期性触发)
		if m.redisStore != nil && m.summarizer != nil && m.config.SummaryTriggerTurns > 0 {
			triggerCount := m.config.SummaryTriggerTurns * 2
			if totalCount > 0 && totalCount%triggerCount == 0 {
				existingSummary, _ := m.redisStore.GetSummary(ctx, sessionID)
				recentMsgs, _ := m.redisStore.GetRecentMessages(ctx, sessionID, m.config.ShortTermWindow*2)
				newSummary, sErr := m.summarizer.Summarize(ctx, existingSummary, recentMsgs)
				if sErr == nil && newSummary != "" {
					_ = m.redisStore.SetSummary(ctx, sessionID, newSummary)
					_ = model.ConversationModel.UpdateSummary(ctx, sessionID, newSummary)
					g.Log().Infof(ctx, "[MemoryManager] 会话 %s 增量摘要提炼更新成功: %s", sessionID, newSummary)
				}
			}
		}

		// 4. 将具有沉淀价值的问答提炼后写入 Milvus 长期情节记忆 (使用 Unicode 字符长度校验)
		if m.milvusStore != nil && utf8.RuneCountInString(strings.TrimSpace(userContent)) >= 2 && utf8.RuneCountInString(strings.TrimSpace(assistantContent)) >= 5 {
			memoryText := fmt.Sprintf("【问】%s\n【答】%s", userContent, assistantContent)
			memItem := &MemoryItem{
				SessionID: sessionID,
				UserID:    userID,
				Content:   memoryText,
				CreatedAt: now,
			}
			if err := m.milvusStore.StoreLongTerm(ctx, []*MemoryItem{memItem}); err != nil {
				g.Log().Warningf(ctx, "[MemoryManager] 异步写入 Milvus 长期记忆库失败: %v", err)
			}
		}
	}()
}
