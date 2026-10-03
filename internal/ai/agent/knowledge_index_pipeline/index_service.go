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
	"github.com/gogf/gf/v2/frame/g"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
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

	collectionName := config.MilvusCollectionName
	if v, _ := g.Cfg().Get(ctx, "milvus.collection_name"); !v.IsEmpty() {
		collectionName = v.String()
	}

	cleanIdent := filepath.ToSlash(identifier)
	expr := fmt.Sprintf(`metadata["_source"] like "%%%s%%"`, cleanIdent)
	queryResult, err := cli.Query(ctx, milvusclient.NewQueryOption(collectionName).WithFilter(expr).WithOutputFields("id"))
	if err != nil {
		return err
	}

	idCol := queryResult.GetColumn("id")
	if idCol != nil && idCol.Len() > 0 {
		var idsToDelete []string
		for i := 0; i < idCol.Len(); i++ {
			id, err := idCol.GetAsString(i)
			if err == nil {
				idsToDelete = append(idsToDelete, id)
			}
		}

		if len(idsToDelete) > 0 {
			deleteExpr := fmt.Sprintf(`id in ["%s"]`, strings.Join(idsToDelete, `","`))
			_, err := cli.Delete(ctx, milvusclient.NewDeleteOption(collectionName).WithExpr(deleteExpr))
			if err != nil {
				return err
			}
			fmt.Printf("[info] deleted %d existing records matching: %s\n", len(idsToDelete), cleanIdent)
		}
	}
	return nil
}
