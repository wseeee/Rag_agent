package model

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	cfg "SuperBizAgent/internal/config"
	"SuperBizAgent/pkg/client"
)

func getTestDSN() string {
	if dsn := os.Getenv("TEST_MYSQL_DSN"); dsn != "" {
		return dsn
	}
	if cfg.C.MySQL.DSN != "" {
		return cfg.C.MySQL.DSN
	}
	return "root:password@tcp(127.0.0.1:3307)/superbiz_agent?charset=utf8mb4&parseTime=True&loc=Local"
}

func TestModelLayer(t *testing.T) {
	_, err := client.InitMySQL(getTestDSN())
	if err != nil {
		t.Skipf("skip model layer test, cannot connect mysql: %v", err)
	}

	ctx := context.Background()
	convDAO := ConversationModel
	msgDAO := MessageModel
	docDAO := DocumentModel

	testConvID := fmt.Sprintf("test_model_%d", time.Now().UnixNano())
	testUser := "test_user_model"

	// 1. 测试会话创建与查询
	conv, err := convDAO.GetOrCreate(ctx, testConvID, testUser, "模型层测试会话")
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}
	if conv.ID != testConvID {
		t.Fatalf("expected ID %s, got %s", testConvID, conv.ID)
	}

	// 2. 测试消息插入流水
	newCount, err := msgDAO.SaveInteraction(ctx, testConvID, testUser, "测试提问：Redis key 过期时间怎么查？", "使用 TTL 命令查看剩余秒数。", 20, 30, `{"type":"qa"}`)
	if err != nil {
		t.Fatalf("SaveInteraction failed: %v", err)
	}
	if newCount != 2 {
		t.Fatalf("expected newCount 2, got %d", newCount)
	}

	// 3. 测试查询最近消息
	msgs, err := msgDAO.GetRecentMessages(ctx, testConvID, 10)
	if err != nil {
		t.Fatalf("GetRecentMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	// 4. 测试文档操作
	testMD5 := fmt.Sprintf("md5_%d", time.Now().UnixNano())
	doc := &KnowledgeDocument{
		FileMD5:     testMD5,
		FileName:    "test_handbook.pdf",
		FileSize:    10240,
		FileExt:     "pdf",
		MinioBucket: "superbiz-documents",
		MinioKey:    "docs/test_handbook.pdf",
		Status:      "UPLOADED",
		UserID:      testUser,
	}
	err = docDAO.Upsert(ctx, doc)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	err = docDAO.UpdateStatus(ctx, testMD5, "COMPLETED", "", 12)
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	savedDoc, err := docDAO.GetByMD5(ctx, testMD5)
	if err != nil || savedDoc.Status != "COMPLETED" || savedDoc.ChunkCount != 12 {
		t.Fatalf("unexpected document status or chunk count: %v, doc: %+v", err, savedDoc)
	}

	// 5. 清理
	_ = convDAO.Delete(ctx, testConvID)
	_ = docDAO.Delete(ctx, testMD5)
}
