package memory

import (
	"SuperBizAgent/internal/model"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisStore(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
	})
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("skip redis test, cannot connect: %v", err)
	}

	store := NewRedisStore(client, 1*time.Hour, 10)
	convID := fmt.Sprintf("test_redis_%d", time.Now().UnixNano())

	// 1. 写入消息
	msg1 := &model.Message{ConversationID: convID, Role: "user", Content: "你好，请问如何排查集群故障？", CreatedAt: time.Now()}
	msg2 := &model.Message{ConversationID: convID, Role: "assistant", Content: "你可以通过查看日志和监控指标进行排查。", CreatedAt: time.Now()}
	err := store.PushMessages(ctx, convID, []*model.Message{msg1, msg2})
	if err != nil {
		t.Fatalf("PushMessages failed: %v", err)
	}

	// 2. 读取最近消息
	msgs, err := store.GetRecentMessages(ctx, convID, 10)
	if err != nil {
		t.Fatalf("GetRecentMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("unexpected message roles: %s, %s", msgs[0].Role, msgs[1].Role)
	}

	// 3. 摘要测试
	summaryText := "会话已确认故障排查方向为日志与监控"
	err = store.SetSummary(ctx, convID, summaryText)
	if err != nil {
		t.Fatalf("SetSummary failed: %v", err)
	}

	sum, err := store.GetSummary(ctx, convID)
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}
	if sum != summaryText {
		t.Fatalf("expected summary %q, got %q", summaryText, sum)
	}

	// 4. 清理
	_ = store.ClearSession(ctx, convID)
}
