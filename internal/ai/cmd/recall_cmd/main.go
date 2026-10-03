package main

import (
	retriever2 "SuperBizAgent/internal/ai/retriever"
	"context"
	"fmt"
)

func main() {
	ctx := context.Background()
	r, err := retriever2.NewMilvusRetriever(ctx)
	if err != nil {
		panic(err)
	}

	testQueries := []string{
		"869830db-a055-4479-963b-3c898d27e755", // UUID 精确匹配
		"12000000001",                           // 错误码精确匹配
		"服务下线是什么原因",                           // 语义自然语言检索
	}

	for _, query := range testQueries {
		fmt.Printf("\n========================================\n")
		fmt.Printf("🔍 测试查询: %s\n", query)
		docs, err := r.Retrieve(ctx, query)
		if err != nil {
			fmt.Printf("❌ 检索失败: %v\n", err)
			continue
		}
		fmt.Printf("✅ 召回文档数: %d\n", len(docs))
		for i, doc := range docs {
			if i >= 3 {
				break
			}
			preview := doc.Content
			if len([]rune(preview)) > 80 {
				preview = string([]rune(preview)[:80]) + "..."
			}
			fmt.Printf("  [Top %d] ID: %s | 内容: %s\n", i+1, doc.ID, preview)
		}
	}
	fmt.Printf("\n========================================\n")
	fmt.Println("🎉 所有测试查询执行完毕！")
}
