package embedder

import (
	"SuperBizAgent/internal/config"
	"context"
	"log"

	"github.com/cloudwego/eino-ext/components/embedding/dashscope"
	"github.com/cloudwego/eino/components/embedding"
)

type batchingEmbedder struct {
	inner     embedding.Embedder
	batchSize int
}

func DoubaoEmbedding(ctx context.Context) (eb embedding.Embedder, err error) {
	conf := config.C.DoubaoEmbeddingModel
	dim := 2048
	embedder, err := dashscope.NewEmbedder(ctx, &dashscope.EmbeddingConfig{
		Model:      conf.Model,
		APIKey:     conf.ApiKey,
		Dimensions: &dim,
	})
	if err != nil {
		log.Printf("new embedder error: %v\n", err)
		return nil, err
	}
	return &batchingEmbedder{
		inner:     embedder,
		batchSize: 10,
	}, nil
}

func (b *batchingEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if len(texts) <= b.batchSize {
		return b.inner.EmbedStrings(ctx, texts, opts...)
	}
	allEmbeddings := make([][]float64, 0, len(texts))
	for i := 0; i < len(texts); i += b.batchSize {
		end := i + b.batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]
		res, err := b.inner.EmbedStrings(ctx, batch, opts...)
		if err != nil {
			return nil, err
		}
		allEmbeddings = append(allEmbeddings, res...)
	}
	return allEmbeddings, nil
}
