package retriever

import (
	"SuperBizAgent/internal/ai/embedder"
	cfg "SuperBizAgent/internal/config"
	"SuperBizAgent/pkg/client"
	"context"

	"github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino/components/retriever"
)

func NewMilvusRetriever(ctx context.Context) (rtr retriever.Retriever, err error) {
	cli, err := client.NewMilvusClient(ctx)
	if err != nil {
		return nil, err
	}
	eb, err := embedder.DoubaoEmbedding(ctx)
	if err != nil {
		return nil, err
	}
	r, err := milvus.NewRetriever(ctx, &milvus.RetrieverConfig{
		Client:      cli,
		Collection:  cfg.MilvusCollectionName,
		VectorField: "vector",
		OutputFields: []string{
			"id",
			"content",
			"metadata",
		},
		TopK:      1,
		Embedding: eb,
	})
	if err != nil {
		return nil, err
	}
	return r, nil
}
