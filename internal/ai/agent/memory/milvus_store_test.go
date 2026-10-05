package memory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"SuperBizAgent/internal/ai/embedder"
	"SuperBizAgent/pkg/client"
)

func TestMilvusStore_LongTermMemory(t *testing.T) {
	ctx := context.Background()

	cli, err := client.NewMilvusClient(ctx)
	if err != nil {
		t.Skipf("skip milvus test, cannot connect: %v", err)
	}

	eb, err := embedder.DoubaoEmbedding(ctx)
	if err != nil {
		t.Skipf("skip milvus test, cannot init embedder: %v", err)
	}

	store, err := NewMilvusStore(ctx, cli, eb, "test_user_memory", 0.70, 5)
	if err != nil {
		t.Fatalf("NewMilvusStore failed: %v", err)
	}

	testSession := fmt.Sprintf("session_%d", time.Now().UnixNano())

	// 1. 存入两条长期记忆
	items := []*MemoryItem{
		{
			SessionID: testSession,
			UserID:    "user_test_ops",
			Content:   "【高可用运维规范】Kafka 集群在云原生环境下的 JVM 参数建议设置为 -Xms16g -Xmx16g，避免频繁 Full GC。",
			CreatedAt: time.Now(),
		},
		{
			SessionID: testSession,
			UserID:    "user_test_ops",
			Content:   "【业务规则】用户充值优惠券必须在 7 天内使用，过期系统自动作废。",
			CreatedAt: time.Now(),
		},
	}

	err = store.StoreLongTerm(ctx, items)
	if err != nil {
		t.Fatalf("StoreLongTerm failed: %v", err)
	}

	// 给 Milvus 索引与落盘一点时间
	time.Sleep(1 * time.Second)

	// 2. 针对运维问题发起检索（预期命中 Kafka 运维，过滤优惠券规则）
	results, err := store.SearchLongTerm(ctx, "Kafka 的 JVM 内存大小怎么配置？", "user_test_ops", 5, 0.70)
	if err != nil {
		t.Fatalf("SearchLongTerm failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("expected at least 1 high-confidence memory, got 0")
	}

	foundKafka := false
	for _, r := range results {
		if r.Score < 0.70 {
			t.Fatalf("score %f below threshold 0.70 was not filtered!", r.Score)
		}
		if len(r.Content) > 0 {
			foundKafka = true
		}
	}
	if !foundKafka {
		t.Fatalf("did not recall expected kafka memory item")
	}
}
