package client

import (
	cfg "SuperBizAgent/internal/config"
	"context"
	"fmt"

	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// NewMilvusClient 初始化或获取 Milvus 2.5 原生客户端，并确保存储 Collection 具备内置 BM25 Function 与稀疏索引
func NewMilvusClient(ctx context.Context) (*milvusclient.Client, error) {
	addr := cfg.C.Milvus.Addr
	defaultDB := cfg.C.Milvus.DefaultDB
	dbName := cfg.C.Milvus.DBName
	collectionName := cfg.C.Milvus.CollectionName

	// 1. 先连接 default 库
	cli, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address: addr,
		DBName:  defaultDB,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to milvus default database: %w", err)
	}

	// 2. 检查并创建业务数据库
	dbs, err := cli.ListDatabase(ctx, milvusclient.NewListDatabaseOption())
	if err != nil {
		return nil, fmt.Errorf("list databases failed: %w", err)
	}
	dbExists := false
	for _, d := range dbs {
		if d == dbName {
			dbExists = true
			break
		}
	}
	if !dbExists {
		if err := cli.CreateDatabase(ctx, milvusclient.NewCreateDatabaseOption(dbName)); err != nil {
			return nil, fmt.Errorf("create database %s failed: %w", dbName, err)
		}
	}

	// 3. 切换至业务数据库
	if err := cli.UseDatabase(ctx, milvusclient.NewUseDatabaseOption(dbName)); err != nil {
		return nil, fmt.Errorf("use database %s failed: %w", dbName, err)
	}

	// 4. 检查集合是否存在，若存在但缺少 sparse_vector 稀疏字段则 Drop 重新初始化
	hasCol, err := cli.HasCollection(ctx, milvusclient.NewHasCollectionOption(collectionName))
	if err != nil {
		return nil, fmt.Errorf("check collection %s failed: %w", collectionName, err)
	}

	needRecreate := !hasCol
	if hasCol {
		colDesc, err := cli.DescribeCollection(ctx, milvusclient.NewDescribeCollectionOption(collectionName))
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
				_ = cli.DropCollection(ctx, milvusclient.NewDropCollectionOption(collectionName))
				needRecreate = true
			}
		}
	}

	if needRecreate {
		idField := entity.NewField().WithName("id").WithDataType(entity.FieldTypeVarChar).WithMaxLength(256).WithIsPrimaryKey(true)
		vectorField := entity.NewField().WithName("vector").WithDataType(entity.FieldTypeFloatVector).WithDim(2048)
		contentField := entity.NewField().WithName("content").WithDataType(entity.FieldTypeVarChar).WithMaxLength(8192).WithEnableAnalyzer(true).WithEnableMatch(true)
		sparseField := entity.NewField().WithName("sparse_vector").WithDataType(entity.FieldTypeSparseVector)
		metaField := entity.NewField().WithName("metadata").WithDataType(entity.FieldTypeJSON)

		bm25Func := entity.NewFunction().
			WithName("content_bm25").
			WithInputFields("content").
			WithOutputFields("sparse_vector").
			WithType(entity.FunctionTypeBM25)

		schema := entity.NewSchema().
			WithName(collectionName).
			WithField(idField).
			WithField(vectorField).
			WithField(contentField).
			WithField(sparseField).
			WithField(metaField).
			WithFunction(bm25Func)

		hnswIdx := index.NewHNSWIndex(entity.COSINE, 16, 64)
		sparseIdx := index.NewSparseInvertedIndex(entity.BM25, 0.2)

		createOpt := milvusclient.NewCreateCollectionOption(collectionName, schema).
			WithIndexOptions(
				milvusclient.NewCreateIndexOption(collectionName, "vector", hnswIdx),
				milvusclient.NewCreateIndexOption(collectionName, "sparse_vector", sparseIdx),
			)

		if err := cli.CreateCollection(ctx, createOpt); err != nil {
			return nil, fmt.Errorf("create collection %s with BM25 function failed: %w", collectionName, err)
		}
	}

	// 5. 确保集合加载到内存供检索
	loadTask, err := cli.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(collectionName))
	if err == nil {
		_ = loadTask.Await(ctx)
	}

	return cli, nil
}
