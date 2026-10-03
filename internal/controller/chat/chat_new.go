package chat

import (
	"context"

	"SuperBizAgent/api/chat"
	"SuperBizAgent/internal/config"
	"SuperBizAgent/internal/logic/sse"
	"SuperBizAgent/pkg/bitmap"
	"SuperBizAgent/pkg/kafka"
	"SuperBizAgent/pkg/storage"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/redis/go-redis/v9"
)

type ControllerV1 struct {
	service       *sse.Service
	bitmapManager *bitmap.ChunkBitmap
	minioStorage  *storage.MinioStorage
	kafkaProducer *kafka.TaskProducer
}

func NewV1() chat.IChatV1 {
	ctx := context.Background()

	// 1. 初始化 Redis Bitmap 管理器
	redisAddr := config.DefaultRedisAddr
	if v, _ := g.Cfg().Get(ctx, "redis.addr"); !v.IsEmpty() {
		redisAddr = v.String()
	}
	redisPassword := config.DefaultRedisPassword
	if v, _ := g.Cfg().Get(ctx, "redis.password"); !v.IsEmpty() {
		redisPassword = v.String()
	}
	redisDB := config.DefaultRedisDB
	if v, _ := g.Cfg().Get(ctx, "redis.db"); !v.IsEmpty() {
		redisDB = v.Int()
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})
	bm := bitmap.NewChunkBitmap(rdb)

	// 2. 初始化 MinIO 对象存储管理器
	minioEndpoint := config.DefaultMinIOEndpoint
	if v, _ := g.Cfg().Get(ctx, "minio.endpoint"); !v.IsEmpty() {
		minioEndpoint = v.String()
	}
	minioAccessKey := config.DefaultMinIOAccessKey
	if v, _ := g.Cfg().Get(ctx, "minio.accessKey"); !v.IsEmpty() {
		minioAccessKey = v.String()
	}
	minioSecretKey := config.DefaultMinIOSecretKey
	if v, _ := g.Cfg().Get(ctx, "minio.secretKey"); !v.IsEmpty() {
		minioSecretKey = v.String()
	}
	minioBucket := config.DefaultMinIOBucket
	if v, _ := g.Cfg().Get(ctx, "minio.bucket"); !v.IsEmpty() {
		minioBucket = v.String()
	}
	minioUseSSL := false
	if v, _ := g.Cfg().Get(ctx, "minio.useSSL"); !v.IsEmpty() {
		minioUseSSL = v.Bool()
	}
	ms, err := storage.NewMinioStorage(minioEndpoint, minioAccessKey, minioSecretKey, minioBucket, minioUseSSL)
	if err != nil {
		g.Log().Warningf(ctx, "init MinIO storage failed: %v", err)
	}

	// 3. 初始化 Kafka 生产者
	kafkaBrokers := []string{config.DefaultKafkaBroker}
	if v, _ := g.Cfg().Get(ctx, "kafka.brokers"); !v.IsEmpty() {
		kafkaBrokers = v.Strings()
	}
	kafkaTopic := config.DefaultKafkaTopic
	if v, _ := g.Cfg().Get(ctx, "kafka.topic"); !v.IsEmpty() {
		kafkaTopic = v.String()
	}
	kp := kafka.NewTaskProducer(kafkaBrokers, kafkaTopic)

	return &ControllerV1{
		service:       sse.New(),
		bitmapManager: bm,
		minioStorage:  ms,
		kafkaProducer: kp,
	}
}

// NewV1WithDeps 提供依赖注入构造器，方便测试与自定义装配
func NewV1WithDeps(
	service *sse.Service,
	bm *bitmap.ChunkBitmap,
	ms *storage.MinioStorage,
	kp *kafka.TaskProducer,
) *ControllerV1 {
	return &ControllerV1{
		service:       service,
		bitmapManager: bm,
		minioStorage:  ms,
		kafkaProducer: kp,
	}
}

