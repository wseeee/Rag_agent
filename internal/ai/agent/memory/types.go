package memory

import (
	"time"

	"SuperBizAgent/internal/model"
)

// MemoryItem Milvus 长期记忆实体
type MemoryItem struct {
	ID        int64     `json:"id"`
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	Content   string    `json:"content"`
	Score     float32   `json:"score"` // 相似度分值 (Cosine 相似度)
	CreatedAt time.Time `json:"created_at"`
}

// MemoryConfig 分层记忆系统配置
type MemoryConfig struct {
	ShortTermWindow     int           `json:"short_term_window"`     // 短期记忆保留轮次(默认 10 轮)
	SummaryTriggerTurns int           `json:"summary_trigger_turns"` // 触发增量摘要轮次(默认 10 轮)
	TTL                 time.Duration `json:"ttl"`                   // Redis 缓存过期时间(默认 7 天)
	ScoreThreshold      float32       `json:"score_threshold"`       // Milvus 长期记忆 Cosine 截断阈值(默认 >= 0.70)
	TopK                int           `json:"top_k"`                 // 长期记忆最大召回条数(默认 5)
	MilvusCollection    string        `json:"milvus_collection"`     // Milvus 长期记忆集合名(默认 user_memory)
}

// AssembledContext 提问前装配完毕的上下文数据
type AssembledContext struct {
	Summary          string           `json:"summary"`            // 阶段摘要
	LongTermMemories []*MemoryItem    `json:"long_term_memories"` // 阈值过滤后的长期记忆
	RecentMessages   []*model.Message `json:"recent_messages"`    // 近轮原始问答
}
