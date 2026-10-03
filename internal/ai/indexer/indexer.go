package indexer

import (
	embedder2 "SuperBizAgent/internal/ai/embedder"
	cfg "SuperBizAgent/internal/config"
	"SuperBizAgent/pkg/client"
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

type milvusNativeIndexer struct {
	cli        *milvusclient.Client
	collection string
	eb         embedding.Embedder
}

func NewMilvusIndexer(ctx context.Context) (indexer.Indexer, error) {
	cli, err := client.NewMilvusClient(ctx)
	if err != nil {
		return nil, err
	}
	eb, err := embedder2.DoubaoEmbedding(ctx)
	if err != nil {
		return nil, err
	}
	collection := cfg.MilvusCollectionName
	if v, _ := g.Cfg().Get(ctx, "milvus.collection_name"); !v.IsEmpty() {
		collection = v.String()
	}
	return &milvusNativeIndexer{
		cli:        cli,
		collection: collection,
		eb:         eb,
	}, nil
}

func (m *milvusNativeIndexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) ([]string, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	total := len(docs)
	ids := make([]string, total)
	contents := make([]string, total)
	metaBytes := make([][]byte, total)

	for i, d := range docs {
		id := d.ID
		if id == "" {
			id = uuid.NewString()
			d.ID = id
		}
		ids[i] = id
		contents[i] = d.Content
		bs, _ := json.Marshal(d.MetaData)
		metaBytes[i] = bs
	}

	// 1. 生成 2048 维 Dense 稠密向量
	vectorsFloat, err := m.eb.EmbedStrings(ctx, contents)
	if err != nil {
		return nil, fmt.Errorf("embed documents failed: %w", err)
	}

	vectors := make([][]float32, total)
	for i, v := range vectorsFloat {
		vec32 := make([]float32, len(v))
		for j, val := range v {
			vec32[j] = float32(val)
		}
		vectors[i] = vec32
	}

	// 2. 构造列式写入选项（注意：sparse_vector 由 Milvus 服务端内置 BM25 Function 自动计算生成！）
	opt := milvusclient.NewColumnBasedInsertOption(m.collection).
		WithColumns(
			column.NewColumnVarChar("id", ids),
			column.NewColumnFloatVector("vector", 2048, vectors),
			column.NewColumnVarChar("content", contents),
			column.NewColumnJSONBytes("metadata", metaBytes),
		)

	_, err = m.cli.Insert(ctx, opt)
	if err != nil {
		return nil, fmt.Errorf("insert into milvus failed: %w", err)
	}

	return ids, nil
}
