package knowledge_index_pipeline

import (
	"context"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

type runeSafeChunkTransformer struct {
	headerSplitter document.Transformer
	chunkSize      int
	chunkOverlap   int
}

func (t *runeSafeChunkTransformer) Transform(ctx context.Context, src []*schema.Document, opts ...document.TransformerOption) ([]*schema.Document, error) {
	// 1. 先进行 Markdown 标题划分
	docs, err := t.headerSplitter.Transform(ctx, src, opts...)
	if err != nil {
		docs = src
	}

	var results []*schema.Document
	for _, doc := range docs {
		runes := []rune(doc.Content)
		if len(runes) <= t.chunkSize {
			results = append(results, doc)
			continue
		}

		// 2. 超长文本采用带重叠的滑动窗口切分，保证中文字符安全
		start := 0
		subIdx := 0
		step := t.chunkSize - t.chunkOverlap
		if step <= 0 {
			step = t.chunkSize
		}

		for start < len(runes) {
			end := start + t.chunkSize
			if end > len(runes) {
				end = len(runes)
			}

			chunkText := string(runes[start:end])

			meta := make(map[string]interface{})
			for k, v := range doc.MetaData {
				meta[k] = v
			}
			meta["chunk_index"] = subIdx

			results = append(results, &schema.Document{
				ID:       uuid.New().String(),
				Content:  chunkText,
				MetaData: meta,
			})

			if end >= len(runes) {
				break
			}
			start += step
			subIdx++
		}
	}
	return results, nil
}

// newDocumentTransformer component initialization function of node 'MarkdownSplitter' in graph 'KnowledgeIndexing'
func newDocumentTransformer(ctx context.Context) (tfr document.Transformer, err error) {
	config := &markdown.HeaderConfig{
		Headers: map[string]string{
			"#":   "h1",
			"##":  "h2",
			"###": "h3",
		},
		TrimHeaders: false,
		IDGenerator: func(ctx context.Context, originalID string, splitIndex int) string {
			return uuid.New().String()
		},
	}
	headerSplitter, err := markdown.NewHeaderSplitter(ctx, config)
	if err != nil {
		return nil, err
	}

	return &runeSafeChunkTransformer{
		headerSplitter: headerSplitter,
		chunkSize:      800,
		chunkOverlap:   100,
	}, nil
}
