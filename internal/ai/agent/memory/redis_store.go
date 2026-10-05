package memory

import (
	"SuperBizAgent/internal/model"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore 封装 Redis 短期工作记忆与热缓存
type RedisStore struct {
	client     *redis.Client
	ttl        time.Duration
	windowSize int
}

// NewRedisStore 创建 Redis 缓存存储实例
func NewRedisStore(client *redis.Client, ttl time.Duration, windowSize int) *RedisStore {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	if windowSize <= 0 {
		windowSize = 20
	}
	return &RedisStore{
		client:     client,
		ttl:        ttl,
		windowSize: windowSize,
	}
}

func (r *RedisStore) msgsKey(convID string) string {
	return fmt.Sprintf("sba:mem:%s:msgs", convID)
}

func (r *RedisStore) summaryKey(convID string) string {
	return fmt.Sprintf("sba:mem:%s:summary", convID)
}

// PushMessages 追加多条消息到 Redis List 右侧，并自动续期 TTL 与裁剪最大窗口
func (r *RedisStore) PushMessages(ctx context.Context, convID string, msgs []*model.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	key := r.msgsKey(convID)
	pipe := r.client.Pipeline()

	values := make([]interface{}, 0, len(msgs))
	for _, m := range msgs {
		b, err := json.Marshal(m)
		if err != nil {
			return fmt.Errorf("marshal message failed: %w", err)
		}
		values = append(values, string(b))
	}

	pipe.RPush(ctx, key, values...)
	// 保持固定窗口，超出的老消息自然淘汰（由 Summary 承担其语义压缩）
	pipe.LTrim(ctx, key, int64(-r.windowSize), -1)
	pipe.Expire(ctx, key, r.ttl)

	_, err := pipe.Exec(ctx)
	return err
}

// GetRecentMessages 从 Redis 获取指定会话最近 limit 条消息（按时间升序）
func (r *RedisStore) GetRecentMessages(ctx context.Context, convID string, limit int) ([]*model.Message, error) {
	if limit <= 0 {
		limit = r.windowSize
	}

	key := r.msgsKey(convID)
	rawList, err := r.client.LRange(ctx, key, int64(-limit), -1).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	if len(rawList) == 0 {
		return nil, nil
	}

	msgs := make([]*model.Message, 0, len(rawList))
	for _, raw := range rawList {
		var m model.Message
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			continue
		}
		msgs = append(msgs, &m)
	}
	return msgs, nil
}

// GetMessageCount 获取当前缓存中消息总数
func (r *RedisStore) GetMessageCount(ctx context.Context, convID string) (int64, error) {
	key := r.msgsKey(convID)
	return r.client.LLen(ctx, key).Result()
}

// SetSummary 存储会话累进摘要，并自动续期 TTL
func (r *RedisStore) SetSummary(ctx context.Context, convID string, summary string) error {
	key := r.summaryKey(convID)
	return r.client.Set(ctx, key, summary, r.ttl).Err()
}

// GetSummary 获取会话累进摘要
func (r *RedisStore) GetSummary(ctx context.Context, convID string) (string, error) {
	key := r.summaryKey(convID)
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", err
	}
	return val, nil
}

// ClearSession 清除指定会话的短期消息队列与摘要
func (r *RedisStore) ClearSession(ctx context.Context, convID string) error {
	pipe := r.client.Pipeline()
	pipe.Del(ctx, r.msgsKey(convID))
	pipe.Del(ctx, r.summaryKey(convID))
	_, err := pipe.Exec(ctx)
	return err
}
