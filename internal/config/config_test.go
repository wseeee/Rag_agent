package config_test

import (
	"context"
	"testing"

	"SuperBizAgent/internal/config"
)

func TestConfigLoad(t *testing.T) {
	ctx := context.Background()
	if err := config.Init(ctx); err != nil {
		t.Fatalf("config.Init failed: %v", err)
	}

	c := config.C

	if c.Server.Address != ":6872" {
		t.Errorf("expected Server.Address ':6872', got %q", c.Server.Address)
	}
	if c.FileDir != "./docs" {
		t.Errorf("expected FileDir './docs', got %q", c.FileDir)
	}
	if c.McpUrl != "http://localhost:3000/sse" {
		t.Errorf("expected McpUrl 'http://localhost:3000/sse', got %q", c.McpUrl)
	}
	if c.Redis.Addr != "127.0.0.1:6379" {
		t.Errorf("expected Redis.Addr '127.0.0.1:6379', got %q", c.Redis.Addr)
	}
	if c.MinIO.Bucket != "superbiz-documents" {
		t.Errorf("expected MinIO.Bucket 'superbiz-documents', got %q", c.MinIO.Bucket)
	}
	if len(c.Kafka.Brokers) == 0 || c.Kafka.Brokers[0] != "127.0.0.1:9092" {
		t.Errorf("expected Kafka.Brokers[0] '127.0.0.1:9092', got %v", c.Kafka.Brokers)
	}
	if c.Kafka.Topic != "superbiz-file-processing" {
		t.Errorf("expected Kafka.Topic 'superbiz-file-processing', got %q", c.Kafka.Topic)
	}
	if c.Milvus.Addr != "localhost:19530" {
		t.Errorf("expected Milvus.Addr 'localhost:19530', got %q", c.Milvus.Addr)
	}
	if c.Milvus.CollectionName != "biz" {
		t.Errorf("expected Milvus.CollectionName 'biz', got %q", c.Milvus.CollectionName)
	}
	if c.MySQL.DSN == "" {
		t.Errorf("expected MySQL.DSN not empty")
	}
	if c.Memory.ShortTermWindow != 10 {
		t.Errorf("expected Memory.ShortTermWindow 10, got %d", c.Memory.ShortTermWindow)
	}
	if c.Memory.MilvusCollection != "user_memory" {
		t.Errorf("expected Memory.MilvusCollection 'user_memory', got %q", c.Memory.MilvusCollection)
	}
	if c.DsThinkChatModel.Model != "deepseek-v4.1-flash" {
		t.Errorf("expected DsThinkChatModel.Model 'deepseek-v4.1-flash', got %q", c.DsThinkChatModel.Model)
	}
	if c.DoubaoEmbeddingModel.Model != "text-embedding-v4" {
		t.Errorf("expected DoubaoEmbeddingModel.Model 'text-embedding-v4', got %q", c.DoubaoEmbeddingModel.Model)
	}
}
