package retriever

import (
	"SuperBizAgent/internal/ai/embedder"
	cfg "SuperBizAgent/internal/config"
	"SuperBizAgent/pkg/client"
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

type hybridMilvusRetriever struct {
	cli         *milvusclient.Client
	collection  string
	eb          embedding.Embedder
	topK        int
	denseWeight float64
	bm25Weight  float64
}

// NewMilvusRetriever 创建 Milvus 2.5 原生双引擎混合检索器（2048维语义 Dense + 服务端内置 BM25 Function + 服务端 WeightedRanker 原生重排）
func NewMilvusRetriever(ctx context.Context) (retriever.Retriever, error) {
	cli, err := client.NewMilvusClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("init milvus client failed: %w", err)
	}
	eb, err := embedder.DoubaoEmbedding(ctx)
	if err != nil {
		return nil, fmt.Errorf("init embedding failed: %w", err)
	}

	return &hybridMilvusRetriever{
		cli:         cli,
		collection:  cfg.C.Milvus.CollectionName,
		eb:          eb,
		topK:        cfg.C.Retriever.TopK,
		denseWeight: cfg.C.Retriever.DenseWeight,
		bm25Weight:  cfg.C.Retriever.Bm25Weight,
	}, nil
}

func (h *hybridMilvusRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	// 1. 生成阿里 2048 维 Dense 稠密向量
	embFloats, err := h.eb.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query failed: %w", err)
	}
	if len(embFloats) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}

	queryVec := make([]float32, len(embFloats[0]))
	for i, v := range embFloats[0] {
		queryVec[i] = float32(v)
	}

	// 2. 构建 Dense 向量检索请求（Top-K * 2）
	candidateK := h.topK * 2
	denseReq := milvusclient.NewAnnRequest("vector", candidateK, entity.FloatVector(queryVec))

	// 3. 构建原生 BM25 全文检索请求（传入 entity.Text(query)，Milvus 服务端自动计算 BM25 稀疏权重）
	sparseReq := milvusclient.NewAnnRequest("sparse_vector", candidateK, entity.Text(query))

	// 4. Milvus 2.5 服务端原生 WeightedRanker 融合重排
	reranker := milvusclient.NewWeightedReranker([]float64{h.denseWeight, h.bm25Weight})

	// 5. 单次网络 IO 并发执行双引擎混合检索
	hybridOpt := milvusclient.NewHybridSearchOption(h.collection, h.topK, denseReq, sparseReq).
		WithReranker(reranker).
		WithOutputFields("id", "content", "metadata")

	resultSets, err := h.cli.HybridSearch(ctx, hybridOpt)
	if err != nil {
		return nil, fmt.Errorf("milvus native hybrid search failed: %w", err)
	}

	if len(resultSets) == 0 {
		return nil, nil
	}

	rs := resultSets[0]
	docs := make([]*schema.Document, 0, rs.ResultCount)
	idCol := rs.GetColumn("id")
	contentCol := rs.GetColumn("content")
	metaCol := rs.GetColumn("metadata")

	for i := 0; i < rs.ResultCount; i++ {
		doc := &schema.Document{
			MetaData: make(map[string]interface{}),
		}
		if idCol != nil {
			if id, err := idCol.GetAsString(i); err == nil {
				doc.ID = id
			}
		}
		if contentCol != nil {
			if content, err := contentCol.GetAsString(i); err == nil {
				doc.Content = content
			}
		}
		if metaCol != nil {
			if metaBytes, err := metaCol.Get(i); err == nil {
				if bs, ok := metaBytes.([]byte); ok {
					var m map[string]interface{}
					if json.Unmarshal(bs, &m) == nil {
						doc.MetaData = m
					}
				}
			}
		}
		docs = append(docs, doc)
	}

	return docs, nil
}
