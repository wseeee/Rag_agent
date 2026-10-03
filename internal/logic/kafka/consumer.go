package kafka

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"SuperBizAgent/internal/ai/agent/knowledge_index_pipeline"
	"SuperBizAgent/internal/config"
	pkgKafka "SuperBizAgent/pkg/kafka"
	"SuperBizAgent/pkg/storage"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
	"github.com/redis/go-redis/v9"
)

// IndexingConsumer 负责监听 Kafka 任务队列，异步拉取并索引文档
type IndexingConsumer struct {
	consumer     *pkgKafka.TaskConsumer
	minioStorage *storage.MinioStorage
	redisClient  *redis.Client
}

// NewIndexingConsumer 创建文档索引消费端
func NewIndexingConsumer(
	consumer *pkgKafka.TaskConsumer,
	minioStorage *storage.MinioStorage,
	redisClient *redis.Client,
) *IndexingConsumer {
	return &IndexingConsumer{
		consumer:     consumer,
		minioStorage: minioStorage,
		redisClient:  redisClient,
	}
}

// Start 启动后台监听循环，直到上下文取消
func (c *IndexingConsumer) Start(ctx context.Context) {
	// ponytail: 当前为单 worker 顺序消费，吞吐量瓶颈时增加 Kafka 分区并扩展 worker 协程池
	g.Log().Info(ctx, "[Kafka Consumer] 异步文档索引消费者启动成功，开始监听任务队列...")

	for {
		select {
		case <-ctx.Done():
			g.Log().Info(ctx, "[Kafka Consumer] 收到退出信号，停止消费循环")
			return
		default:
			task, msg, err := c.consumer.FetchTask(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				g.Log().Warningf(ctx, "[Kafka Consumer] 获取任务消息失败: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			g.Log().Infof(ctx, "[Kafka Consumer] 收到文件索引任务: TaskID=%s, FileName=%s, MD5=%s", task.TaskID, task.FileName, task.FileMD5)

			// 执行带指数退避的有限次重试，防止网络瞬断导致任务永久失败
			const maxRetries = 3
			var procErr error
			for attempt := 1; attempt <= maxRetries; attempt++ {
				procErr = c.processTask(ctx, task)
				if procErr == nil {
					break
				}
				g.Log().Warningf(ctx, "[Kafka Consumer] 处理任务失败 (第 %d/%d 次尝试): TaskID=%s, err=%v", attempt, maxRetries, task.TaskID, procErr)
				if attempt < maxRetries {
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Duration(attempt) * time.Second):
					}
				}
			}

			// 若重试耗尽仍然失败，记录严重错误并提交 Offset 避免毒丸消息阻塞队列
			if procErr != nil {
				g.Log().Errorf(ctx, "[Kafka Consumer] 任务处理重试耗尽最终失败: TaskID=%s, err=%v", task.TaskID, procErr)
			}

			if msg != nil {
				if err := c.consumer.Commit(ctx, *msg); err != nil {
					g.Log().Warningf(ctx, "[Kafka Consumer] 提交消息 Offset 失败: %v", err)
				}
			}
		}
	}
}

func (c *IndexingConsumer) processTask(ctx context.Context, task *pkgKafka.FileProcessingTask) error {
	taskKey := fmt.Sprintf("task:%s", task.TaskID)

	// 1. 更新任务状态为 PROCESSING
	if c.redisClient != nil {
		_ = c.redisClient.HSet(ctx, taskKey, map[string]interface{}{
			"status":     "PROCESSING",
			"file_name":  task.FileName,
			"file_md5":   task.FileMD5,
			"started_at": time.Now().UnixMilli(),
		}).Err()
		_ = c.redisClient.Expire(ctx, taskKey, 7*24*time.Hour)
	}

	// 2. 准备本地下载保存目录
	destDir := filepath.Join(config.FileDir, task.FileMD5)
	if !gfile.Exists(destDir) {
		if err := gfile.Mkdir(destDir); err != nil {
			return fmt.Errorf("create download dir failed: %w", err)
		}
	}
	localPath := filepath.Join(destDir, task.FileName)

	// 3. 从 MinIO 下载合并后的完整文件
	if err := c.minioStorage.DownloadObject(ctx, task.MinIOObjectKey, localPath); err != nil {
		c.markFailed(ctx, taskKey, fmt.Sprintf("从 MinIO 下载文件失败: %v", err))
		return err
	}

	// 4. 调用向量化 Pipeline 构建知识库
	ids, err := knowledge_index_pipeline.IndexDocument(ctx, localPath)
	if err != nil {
		c.markFailed(ctx, taskKey, fmt.Sprintf("构建向量索引失败: %v", err))
		return err
	}

	// 5. 更新任务状态为 COMPLETED
	if c.redisClient != nil {
		_ = c.redisClient.HSet(ctx, taskKey, map[string]interface{}{
			"status":       "COMPLETED",
			"doc_count":    len(ids),
			"completed_at": time.Now().UnixMilli(),
		}).Err()
	}

	g.Log().Infof(ctx, "[Kafka Consumer] 文件索引完成: TaskID=%s, 生成向量片段数=%d", task.TaskID, len(ids))
	return nil
}

func (c *IndexingConsumer) markFailed(ctx context.Context, taskKey, errorMsg string) {
	if c.redisClient != nil {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.redisClient.HSet(bgCtx, taskKey, map[string]interface{}{
			"status":    "FAILED",
			"error":     errorMsg,
			"failed_at": time.Now().UnixMilli(),
		}).Err()
	}
}

// Close 优雅释放资源
func (c *IndexingConsumer) Close() error {
	return c.consumer.Close()
}

// InitAndStartConsumer 根据配置快速初始化并启动后台消费监听
func InitAndStartConsumer(ctx context.Context) (*IndexingConsumer, error) {
	// Redis
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

	// MinIO
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
		return nil, fmt.Errorf("init minio failed: %w", err)
	}

	// Kafka
	kafkaBrokers := []string{config.DefaultKafkaBroker}
	if v, _ := g.Cfg().Get(ctx, "kafka.brokers"); !v.IsEmpty() {
		kafkaBrokers = v.Strings()
	}
	kafkaTopic := config.DefaultKafkaTopic
	if v, _ := g.Cfg().Get(ctx, "kafka.topic"); !v.IsEmpty() {
		kafkaTopic = v.String()
	}
	kafkaGroupID := config.DefaultKafkaGroupID
	if v, _ := g.Cfg().Get(ctx, "kafka.group_id"); !v.IsEmpty() {
		kafkaGroupID = v.String()
	}

	consumer := pkgKafka.NewTaskConsumer(kafkaBrokers, kafkaTopic, kafkaGroupID)
	indexingConsumer := NewIndexingConsumer(consumer, ms, rdb)

	go indexingConsumer.Start(ctx)

	return indexingConsumer, nil
}
