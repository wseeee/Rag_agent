package main

import (
	"SuperBizAgent/internal/ai/agent/knowledge_index_pipeline"
	loader2 "SuperBizAgent/internal/ai/loader"
	"SuperBizAgent/internal/config"
	"SuperBizAgent/pkg/client"
	"SuperBizAgent/pkg/log_call_back"
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
)

func main() {
	ctx := context.Background()
	r, err := knowledge_index_pipeline.BuildKnowledgeIndexing(ctx)
	if err != nil {
		panic(err)
	}
	err = filepath.WalkDir("./docs", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk dir failed: %w", err)
		}
		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".md") {
			fmt.Printf("[skip] not a markdown file: %s\n", path)
			return nil
		}

		fmt.Printf("[start] indexing file: %s\n", path)
		// 删除biz数据metadata中_source一样的数据
		loader, err := loader2.NewFileLoader(ctx)
		if err != nil {
			return err
		}
		cleanPath := filepath.ToSlash(path)
		docs, err := loader.Load(ctx, document.Source{URI: cleanPath})
		if err != nil {
			return err
		}
		cli, err := client.NewMilvusClient(ctx)
		if err != nil {
			return err
		}
		// 查询所有metadata中_source一样的数据并删除
		source := filepath.ToSlash(fmt.Sprintf("%v", docs[0].MetaData["_source"]))
		expr := fmt.Sprintf(`metadata["_source"] == "%s"`, source)
		queryResult, err := cli.Query(ctx, config.MilvusCollectionName, []string{}, expr, []string{"id"})
		if err != nil {
			return err
		} else if len(queryResult) > 0 {
			// 提取所有需要删除的id
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
			// 删除这些数据
			if len(idsToDelete) > 0 {
				deleteExpr := fmt.Sprintf(`id in ["%s"]`, strings.Join(idsToDelete, `","`))
				err = cli.Delete(ctx, config.MilvusCollectionName, "", deleteExpr)
				if err != nil {
					fmt.Printf("[warn] delete existing data failed: %v\n", err)
				} else {
					fmt.Printf("[info] deleted %d existing records with _source: %s\n", len(idsToDelete), docs[0].MetaData["_source"])
				}
			}
		}
		// 重新构建
		ids, err := r.Invoke(ctx, document.Source{URI: cleanPath}, compose.WithCallbacks(log_call_back.LogCallback(nil)))
		if err != nil {
			return fmt.Errorf("invoke index graph failed: %w", err)
		}
		fmt.Printf("[done] indexing file: %s, len of parts: %d, ids: %v\n", cleanPath, len(ids), ids)
		return nil
	})
	if err != nil {
		panic(err)
	}
}
