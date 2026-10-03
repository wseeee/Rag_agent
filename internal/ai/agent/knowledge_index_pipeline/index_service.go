package knowledge_index_pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	loader2 "SuperBizAgent/internal/ai/loader"
	"SuperBizAgent/internal/config"
	"SuperBizAgent/pkg/client"
	"SuperBizAgent/pkg/log_call_back"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
)

// IndexDocument 加载文档、清理 Milvus 中已存在的同源历史向量、执行嵌入与向量索引入库
func IndexDocument(ctx context.Context, path string) ([]string, error) {
	cleanPath := filepath.ToSlash(path)
	r, err := BuildKnowledgeIndexing(ctx)
	if err != nil {
		return nil, fmt.Errorf("build knowledge indexing graph failed: %w", err)
	}

	loader, err := loader2.NewFileLoader(ctx)
	if err != nil {
		return nil, fmt.Errorf("init file loader failed: %w", err)
	}

	docs, err := loader.Load(ctx, document.Source{URI: cleanPath})
	if err != nil {
		return nil, fmt.Errorf("load document failed: %w", err)
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf("no documents loaded from %s", cleanPath)
	}

	cli, err := client.NewMilvusClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("init milvus client failed: %w", err)
	}

	// 查询所有 metadata 中 _source 一样的数据并删除，保证幂等性
	source := filepath.ToSlash(fmt.Sprintf("%v", docs[0].MetaData["_source"]))
	expr := fmt.Sprintf(`metadata["_source"] == "%s"`, source)
	queryResult, err := cli.Query(ctx, config.MilvusCollectionName, []string{}, expr, []string{"id"})
	if err != nil {
		fmt.Printf("[warn] query existing Milvus data failed: %v\n", err)
	} else if len(queryResult) > 0 {
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
				fmt.Printf("[warn] delete existing data failed: %v\n", err)
			} else {
				fmt.Printf("[info] deleted %d existing records with _source: %s\n", len(idsToDelete), source)
			}
		}
	}

	// 重新构建向量索引
	ids, err := r.Invoke(ctx, document.Source{URI: cleanPath}, compose.WithCallbacks(log_call_back.LogCallback(nil)))
	if err != nil {
		return nil, fmt.Errorf("invoke index graph failed: %w", err)
	}

	fmt.Printf("[done] indexing file: %s, len of parts: %d\n", cleanPath, len(ids))
	return ids, nil
}
