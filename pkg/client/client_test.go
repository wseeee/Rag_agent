package client

import (
	"context"
	"os"
	"testing"

	cfg "SuperBizAgent/internal/config"
)

func TestRedisClientInit(t *testing.T) {
	rdb := GetRedisClient()
	if rdb == nil {
		t.Fatal("expected non-nil redis client from GetRedisClient()")
	}

	// 验证单例模式
	rdb2 := GetRedisClient()
	if rdb != rdb2 {
		t.Fatal("expected GetRedisClient() to return singleton instance")
	}

	// 验证 NewRedisClient 可以创建新实例
	rdb3 := NewRedisClient()
	if rdb3 == nil {
		t.Fatal("expected non-nil redis client from NewRedisClient()")
	}
	defer rdb3.Close()

	// 连通性测试 (若本地未启动 Redis 则跳过)
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("skip redis connectivity check: %v", err)
	}
}

func TestMySQLClientInit(t *testing.T) {
	testDSN := os.Getenv("TEST_MYSQL_DSN")
	if testDSN == "" {
		testDSN = cfg.C.MySQL.DSN
	}
	if testDSN == "" {
		testDSN = "root:password@tcp(127.0.0.1:3307)/superbiz_agent?charset=utf8mb4&parseTime=True&loc=Local"
	}
	db, err := InitMySQL(testDSN)
	if err != nil {
		t.Skipf("skip mysql connectivity check: %v", err)
	}

	if db == nil {
		t.Fatal("expected non-nil gorm.DB")
	}

	db2 := GetDB()
	if db2 == nil {
		t.Fatal("expected non-nil gorm.DB from GetDB()")
	}
}

func TestKafkaClientInit(t *testing.T) {
	writer := NewKafkaWriter([]string{"127.0.0.1:9092"}, "test-topic")
	if writer == nil {
		t.Fatal("expected non-nil kafka.Writer")
	}
	defer writer.Close()

	reader := NewKafkaReader([]string{"127.0.0.1:9092"}, "test-topic", "test-group")
	if reader == nil {
		t.Fatal("expected non-nil kafka.Reader")
	}
	defer reader.Close()
}

func TestTikaClientInit(t *testing.T) {
	tc := NewTikaClient("http://127.0.0.1:9998")
	if tc == nil {
		t.Fatal("expected non-nil TikaClient")
	}
	if tc.endpoint != "http://127.0.0.1:9998" {
		t.Fatalf("expected endpoint 'http://127.0.0.1:9998', got '%s'", tc.endpoint)
	}
}
