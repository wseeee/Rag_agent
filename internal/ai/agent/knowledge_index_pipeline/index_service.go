package knowledge_index_pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"SuperBizAgent/internal/config"
	"SuperBizAgent/pkg/client"
	"SuperBizAgent/pkg/log_call_back"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
)

// IndexDocument 清理 Milvus 中已存在的同源历史向量，执行 Eino 流水线完成加载、切分与向量索引入库
func IndexDocument(ctx context.Context, path string) ([]string, error) {
	cleanPath := filepath.ToSlash(path)

	// 1. 保证幂等性：入库前清理旧向量（以 cleanPath 作为 _source 匹配标识）
	if err := DeleteDocumentVectors(ctx, cleanPath); err != nil {
		fmt.Printf("[warn] delete existing Milvus data failed: %v\n", err)
	}

	// 2. 编译知识库编排图
	r, err := BuildKnowledgeIndexing(ctx)
	if err != nil {
		return nil, fmt.Errorf("build knowledge indexing graph failed: %w", err)
	}

	// 3. 执行图流水线（内部 FileLoader 自动加载文件，避免重复 I/O）
	ids, err := r.Invoke(ctx, document.Source{URI: cleanPath}, compose.WithCallbacks(log_call_back.LogCallback(nil)))
	if err != nil {
		return nil, fmt.Errorf("invoke index graph failed: %w", err)
	}

	fmt.Printf("[done] indexing file: %s, len of parts: %d\n", cleanPath, len(ids))
	return ids, nil
}

// DeleteDocumentVectors 根据文件名或文件MD5清理 Milvus 中的关联历史向量
func DeleteDocumentVectors(ctx context.Context, identifier string) error {
	cli, err := client.NewMilvusClient(ctx)
	if err != nil {
		return fmt.Errorf("init milvus client failed: %w", err)
	}

	cleanIdent := filepath.ToSlash(identifier)
	expr := fmt.Sprintf(`metadata["_source"] like "%%%s%%"`, cleanIdent)
	queryResult, err := cli.Query(ctx, config.MilvusCollectionName, []string{}, expr, []string{"id"})
	if err != nil {
		return err
	}

	var idsToDelete []string
	for _, column := range queryResult {
		if column.Name() == "id" {
			for i := 0; i < column.Len(); i++ {
				id, err := column.GetAsString(i)
				if err == nil {
					idsToDelete = append(idsToDelete, id)
				}
			}
		}
	}

	if len(idsToDelete) > 0 {
		deleteExpr := fmt.Sprintf(`id in ["%s"]`, strings.Join(idsToDelete, `","`))
		if err := cli.Delete(ctx, config.MilvusCollectionName, "", deleteExpr); err != nil {
			return err
		}
		fmt.Printf("[info] deleted %d existing records matching: %s\n", len(idsToDelete), cleanIdent)
	}
	return nil
}
