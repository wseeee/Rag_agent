package memory

import (
	cfg "SuperBizAgent/internal/config"
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/google/uuid"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// MilvusStore 封装 Milvus 2.5 长期情节记忆库操作
type MilvusStore struct {
	cli            *milvusclient.Client
	collection     string
	eb             embedding.Embedder
	scoreThreshold float32
	topK           int
}

// NewMilvusStore 初始化 Milvus 记忆集合与长期记忆存储器
func NewMilvusStore(
	ctx context.Context,
	cli *milvusclient.Client,
	eb embedding.Embedder,
	collection string,
	scoreThreshold float32,
	topK int,
) (*MilvusStore, error) {
	store := &MilvusStore{
		cli:            cli,
		collection:     collection,
		eb:             eb,
		scoreThreshold: scoreThreshold,
		topK:           topK,
	}

	if err := store.EnsureCollection(ctx); err != nil {
		return nil, fmt.Errorf("ensure milvus memory collection failed: %w", err)
	}

	return store, nil
}

// EnsureCollection 幂等检查并初始化 user_memory 集合（带内置 BM25 Function 与混合检索索引）
func (m *MilvusStore) EnsureCollection(ctx context.Context) error {
	hasCol, err := m.cli.HasCollection(ctx, milvusclient.NewHasCollectionOption(m.collection))
	if err != nil {
		return fmt.Errorf("check collection %s failed: %w", m.collection, err)
	}

	needRecreate := !hasCol
	if hasCol {
		colDesc, err := m.cli.DescribeCollection(ctx, milvusclient.NewDescribeCollectionOption(m.collection))
		if err == nil && colDesc != nil && colDesc.Schema != nil {
			hasSparse := false
			for _, f := range colDesc.Schema.Fields {
				if f.Name == "sparse_vector" {
					hasSparse = true
					break
				}
			}
			hasFunc := len(colDesc.Schema.Functions) > 0
			if !hasSparse || !hasFunc {
				_ = m.cli.DropCollection(ctx, milvusclient.NewDropCollectionOption(m.collection))
				needRecreate = true
			}
		}
	}

	if needRecreate {
		idField := entity.NewField().WithName("id").WithDataType(entity.FieldTypeVarChar).WithMaxLength(64).WithIsPrimaryKey(true)
		sessionField := entity.NewField().WithName("session_id").WithDataType(entity.FieldTypeVarChar).WithMaxLength(64)
		userField := entity.NewField().WithName("user_id").WithDataType(entity.FieldTypeVarChar).WithMaxLength(64)
		contentField := entity.NewField().WithName("content").WithDataType(entity.FieldTypeVarChar).WithMaxLength(8192).WithEnableAnalyzer(true).WithEnableMatch(true)
		vectorField := entity.NewField().WithName("vector").WithDataType(entity.FieldTypeFloatVector).WithDim(2048)
		sparseField := entity.NewField().WithName("sparse_vector").WithDataType(entity.FieldTypeSparseVector)
		metaField := entity.NewField().WithName("metadata").WithDataType(entity.FieldTypeJSON)
		createdAtField := entity.NewField().WithName("created_at").WithDataType(entity.FieldTypeInt64)

		bm25Func := entity.NewFunction().
			WithName("content_bm25").
			WithInputFields("content").
			WithOutputFields("sparse_vector").
			WithType(entity.FunctionTypeBM25)

		schema := entity.NewSchema().
			WithName(m.collection).
			WithField(idField).
			WithField(sessionField).
			WithField(userField).
			WithField(contentField).
			WithField(vectorField).
			WithField(sparseField).
			WithField(metaField).
			WithField(createdAtField).
			WithFunction(bm25Func)

		hnswIdx := index.NewHNSWIndex(entity.COSINE, 16, 64)
		sparseIdx := index.NewSparseInvertedIndex(entity.BM25, 0.2)

		createOpt := milvusclient.NewCreateCollectionOption(m.collection, schema).
			WithIndexOptions(
				milvusclient.NewCreateIndexOption(m.collection, "vector", hnswIdx),
				milvusclient.NewCreateIndexOption(m.collection, "sparse_vector", sparseIdx),
			)

		if err := m.cli.CreateCollection(ctx, createOpt); err != nil {
			return fmt.Errorf("create memory collection %s failed: %w", m.collection, err)
		}
	}

	loadTask, err := m.cli.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(m.collection))
	if err == nil {
		_ = loadTask.Await(ctx)
	}
	return nil
}

// StoreLongTerm 异步或同步将对话知识/偏好提炼项写入 Milvus 长期情节记忆
func (m *MilvusStore) StoreLongTerm(ctx context.Context, items []*MemoryItem) error {
	if len(items) == 0 {
		return nil
	}

	total := len(items)
	ids := make([]string, total)
	sessionIDs := make([]string, total)
	userIDs := make([]string, total)
	contents := make([]string, total)
	createdAts := make([]int64, total)
	metaBytes := make([][]byte, total)

	for i, it := range items {
		ids[i] = uuid.NewString()
		sessionIDs[i] = it.SessionID
		userIDs[i] = it.UserID
		if userIDs[i] == "" {
			userIDs[i] = "default_user"
		}
		contents[i] = it.Content
		if it.CreatedAt.IsZero() {
			createdAts[i] = time.Now().Unix()
		} else {
			createdAts[i] = it.CreatedAt.Unix()
		}
		metaBytes[i] = []byte("{}")
	}

	// 1. 生成 2048 维 Dense 稠密向量
	vectorsFloat, err := m.eb.EmbedStrings(ctx, contents)
	if err != nil {
		return fmt.Errorf("embed memory items failed: %w", err)
	}

	vectors := make([][]float32, total)
	for i, v := range vectorsFloat {
		vec32 := make([]float32, len(v))
		for j, val := range v {
			vec32[j] = float32(val)
		}
		vectors[i] = vec32
	}

	// 2. 列式写入 (sparse_vector 由服务端 BM25 Function 自动计算生成)
	opt := milvusclient.NewColumnBasedInsertOption(m.collection).
		WithColumns(
			column.NewColumnVarChar("id", ids),
			column.NewColumnVarChar("session_id", sessionIDs),
			column.NewColumnVarChar("user_id", userIDs),
			column.NewColumnVarChar("content", contents),
			column.NewColumnFloatVector("vector", 2048, vectors),
			column.NewColumnJSONBytes("metadata", metaBytes),
			column.NewColumnInt64("created_at", createdAts),
		)

	_, err = m.cli.Insert(ctx, opt)
	if err != nil {
		return fmt.Errorf("insert memory into milvus failed: %w", err)
	}

	return nil
}

// SearchLongTerm 发起双引擎混合检索，并严格执行 Score 阈值截断 (过滤 score < threshold 的弱相关噪音)
func (m *MilvusStore) SearchLongTerm(
	ctx context.Context,
	query string,
	userID string,
	topK int,
	threshold float32,
) ([]*MemoryItem, error) {
	if query == "" {
		return nil, nil
	}

	// 1. 生成阿里 2048 维 Query 向量
	embFloats, err := m.eb.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query failed: %w", err)
	}
	if len(embFloats) == 0 {
		return nil, fmt.Errorf("empty query embedding")
	}

	queryVec := make([]float32, len(embFloats[0]))
	for i, v := range embFloats[0] {
		queryVec[i] = float32(v)
	}

	// 2. Dense + Sparse 混合检索请求
	candidateK := topK * 2
	denseReq := milvusclient.NewAnnRequest("vector", candidateK, entity.FloatVector(queryVec))
	sparseReq := milvusclient.NewAnnRequest("sparse_vector", candidateK, entity.Text(query))

	if userID != "" {
		expr := fmt.Sprintf(`user_id == "%s"`, userID)
		denseReq = denseReq.WithFilter(expr)
		sparseReq = sparseReq.WithFilter(expr)
	}

	reranker := milvusclient.NewWeightedReranker([]float64{cfg.C.Retriever.DenseWeight, cfg.C.Retriever.Bm25Weight})
	hybridOpt := milvusclient.NewHybridSearchOption(m.collection, topK, denseReq, sparseReq).
		WithReranker(reranker).
		WithOutputFields("id", "session_id", "user_id", "content", "created_at")

	resultSets, err := m.cli.HybridSearch(ctx, hybridOpt)
	if err != nil {
		return nil, fmt.Errorf("milvus hybrid search failed: %w", err)
	}

	var memories []*MemoryItem
	for _, res := range resultSets {
		contentCol := res.GetColumn("content")
		sessionCol := res.GetColumn("session_id")
		userCol := res.GetColumn("user_id")
		createdAtCol := res.GetColumn("created_at")

		for i := 0; i < res.ResultCount; i++ {
			score := res.Scores[i]
			// 核心过滤：严格过滤相似度低于阈值的噪音
			if score < threshold {
				continue
			}

			var content, sessionID, uID string
			var createdAtInt int64

			if contentCol != nil {
				content, _ = contentCol.GetAsString(i)
			}
			if sessionCol != nil {
				sessionID, _ = sessionCol.GetAsString(i)
			}
			if userCol != nil {
				uID, _ = userCol.GetAsString(i)
			}
			if createdAtCol != nil {
				createdAtInt, _ = createdAtCol.GetAsInt64(i)
			}

			mem := &MemoryItem{
				SessionID: sessionID,
				UserID:    uID,
				Content:   content,
				Score:     score,
			}
			if createdAtInt > 0 {
				mem.CreatedAt = time.Unix(createdAtInt, 0)
			} else {
				mem.CreatedAt = time.Now()
			}

			memories = append(memories, mem)
		}
	}

	return memories, nil
}
