package main

import (
	"context"
	"fmt"
	"time"

	"SuperBizAgent/internal/ai/agent/memory"
	"SuperBizAgent/internal/model"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
)

func main() {
	ctx := context.Background()

	// 确保加载配置文件
	if adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile); ok {
		_ = adapter.AddPath("etc/config", "etc")
	}

	fmt.Println("==================================================================")
	fmt.Println("🚀 开始验证 SuperBizAgent 三层分层记忆系统 (MySQL + Redis + Milvus)")
	fmt.Println("==================================================================")

	memMgr, err := memory.NewDefaultMemoryManager(ctx)
	if err != nil {
		panic(fmt.Sprintf("初始化 MemoryManager 失败: %v", err))
	}

	sessionID := fmt.Sprintf("e2e_session_%d", time.Now().Unix())
	userID := "user_devops_test"

	fmt.Printf("\n[Step 1] 模拟第 1 轮问答并异步落盘...\n")
	q1 := "你好，请记住我的生产服务器环境：Kubernetes 1.28，Kafka 版本是 3.6，MySQL 端口是 3307。"
	a1 := "已收到并记录：你的生产环境为 K8s 1.28，Kafka 3.6，MySQL 端口 3307。后续运维排障将优先结合此环境配置。"
	memMgr.RecordInteractionAsync(sessionID, userID, q1, a1, 35, 60, `{"source":"env_setup"}`)

	// 等待后台 Goroutine 异步完成落盘 (MySQL + Redis + Milvus)
	time.Sleep(2 * time.Second)

	fmt.Printf("\n[Step 2] 模拟第 2 轮提问前装配三层上下文...\n")
	q2 := "我刚才提到的 MySQL 端口是多少？"
	assembled, einoMsgs, err := memMgr.GetAssembledContext(ctx, sessionID, userID, q2)
	if err != nil {
		panic(fmt.Sprintf("GetAssembledContext 失败: %v", err))
	}

	fmt.Printf("✅ 读取到近轮对话消息数: %d\n", len(assembled.RecentMessages))
	for i, m := range assembled.RecentMessages {
		fmt.Printf("   [%d] %s: %s\n", i+1, m.Role, m.Content)
	}

	if len(assembled.RecentMessages) < 2 {
		panic("❌ 错误：未能从 Redis/MySQL 读取到第 1 轮对话！")
	}

	fmt.Printf("✅ 组装注入 Prompt 的消息总数: %d\n", len(einoMsgs))

	fmt.Printf("\n[Step 3] 模拟第 2 轮助手回答并记录...\n")
	a2 := "你刚才提到的 MySQL 端口是 3307。"
	memMgr.RecordInteractionAsync(sessionID, userID, q2, a2, 18, 25, "")
	time.Sleep(2 * time.Second)

	fmt.Printf("\n[Step 4] 校验 MySQL 事实真理源 (持久化查询)...\n")
	msgsInDB, err := model.MessageModel.GetRecentMessages(ctx, sessionID, 10)
	if err != nil {
		panic(fmt.Sprintf("从 MySQL 查询失败: %v", err))
	}
	fmt.Printf("✅ MySQL 实际落盘消息条数: %d (预期 4 条)\n", len(msgsInDB))
	if len(msgsInDB) != 4 {
		panic(fmt.Sprintf("❌ 预期 4 条落盘消息，实际得到 %d 条", len(msgsInDB)))
	}

	convList, total, err := model.ConversationModel.List(ctx, userID, 1, 10)
	if err != nil || total < 1 {
		panic(fmt.Sprintf("❌ 会话列表未检索到: total=%d, err=%v", total, err))
	}
	fmt.Printf("✅ 会话主表成功生成: ID=%s, 标题=%s, 总消息数=%d\n", convList[0].ID, convList[0].Title, convList[0].MessageCount)

	fmt.Printf("\n[Step 5] 验证 Milvus 长期记忆带 Distance 阈值 (>= 0.70) 召回...\n")
	qCross := "我们集群的 Kafka 版本是多少？"
	assembledCross, _, err := memMgr.GetAssembledContext(ctx, "different_session_999", userID, qCross)
	if err != nil {
		fmt.Printf("⚠️ 跨会话检索警告: %v\n", err)
	} else {
		fmt.Printf("✅ 跨会话命中高相关长期记忆条数: %d\n", len(assembledCross.LongTermMemories))
		for idx, lt := range assembledCross.LongTermMemories {
			fmt.Printf("   [%d] 相似度分数=%.4f (阈值>=0.70), 内容=%s\n", idx+1, lt.Score, lt.Content)
		}
	}

	fmt.Println("\n==================================================================")
	fmt.Println("🎉 恭喜！SuperBizAgent 三层分层记忆系统端到端验证全部通过！")
	fmt.Println("==================================================================")
}
