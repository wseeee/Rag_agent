package main

import (
	"SuperBizAgent/internal/ai/agent/knowledge_index_pipeline"
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

func main() {
	ctx := context.Background()
	err := filepath.WalkDir("./docs", func(path string, d fs.DirEntry, err error) error {
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
		cleanPath := filepath.ToSlash(path)
		ids, err := knowledge_index_pipeline.IndexDocument(ctx, cleanPath)
		if err != nil {
			return fmt.Errorf("index document failed: %w", err)
		}
		fmt.Printf("[done] indexed %s with %d chunks: %v\n", cleanPath, len(ids), ids)
		return nil
	})
	if err != nil {
		panic(err)
	}
}
