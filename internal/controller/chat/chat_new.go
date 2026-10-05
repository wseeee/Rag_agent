package chat

import (
	"context"

	"SuperBizAgent/api/chat"
	"SuperBizAgent/internal/config"
	"SuperBizAgent/pkg/bitmap"
	"SuperBizAgent/pkg/client"
	"SuperBizAgent/pkg/sse"
	"SuperBizAgent/pkg/storage"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/segmentio/kafka-go"
)

type ControllerV1 struct {
	service       *sse.Service
	bitmapManager *bitmap.ChunkBitmap
	minioStorage  *storage.MinioStorage
	kafkaWriter   *kafka.Writer
}

func NewV1() chat.IChatV1 {
	ctx := context.Background()

	// 1. 初始化 Redis Bitmap 管理器
	rdb := client.GetRedisClient()
	bm := bitmap.NewChunkBitmap(rdb)

	// 2. 初始化 MinIO 对象存储管理器
	minioCfg := config.C.MinIO
	ms, err := storage.NewMinioStorage(minioCfg.Endpoint, minioCfg.AccessKey, minioCfg.SecretKey, minioCfg.Bucket, minioCfg.UseSSL)
	if err != nil {
		g.Log().Warningf(ctx, "init MinIO storage failed: %v", err)
	}

	// 3. 初始化 Kafka Writer
	kw := client.NewKafkaWriter(config.C.Kafka.Brokers, config.C.Kafka.Topic)

	return &ControllerV1{
		service:       sse.New(),
		bitmapManager: bm,
		minioStorage:  ms,
		kafkaWriter:   kw,
	}
}

// getUserID 优先从 HTTP 请求头提取用户标识，未传递时回退为默认单租户用户
func getUserID(ctx context.Context) string {
	r := g.RequestFromCtx(ctx)
	if r != nil {
		if uid := r.Header.Get("X-User-Id"); uid != "" {
			return uid
		}
	}
	return "default_user"
}

