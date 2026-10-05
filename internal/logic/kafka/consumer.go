package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"SuperBizAgent/internal/ai/agent/knowledge_index_pipeline"
	"SuperBizAgent/internal/config"
	"SuperBizAgent/pkg/client"
	"SuperBizAgent/pkg/storage"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

// IndexingConsumer 负责监听 Kafka 任务队列，异步拉取并索引文档
type IndexingConsumer struct {
	reader       *kafka.Reader
	minioStorage *storage.MinioStorage
	redisClient  *redis.Client
	tikaClient   *client.TikaClient
}

// NewIndexingConsumer 创建文档索引消费端
func NewIndexingConsumer(
	reader *kafka.Reader,
	minioStorage *storage.MinioStorage,
	redisClient *redis.Client,
	tikaClient *client.TikaClient,
) *IndexingConsumer {
	return &IndexingConsumer{
		reader:       reader,
		minioStorage: minioStorage,
		redisClient:  redisClient,
		tikaClient:   tikaClient,
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
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				g.Log().Warningf(ctx, "[Kafka Consumer] 获取任务消息失败: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			var task client.FileProcessingTask
			if err := json.Unmarshal(msg.Value, &task); err != nil {
				g.Log().Errorf(ctx, "[Kafka Consumer] 反序列化任务载荷失败: %v", err)
				_ = c.reader.CommitMessages(ctx, msg)
				continue
			}

			g.Log().Infof(ctx, "[Kafka Consumer] 收到文件索引任务: TaskID=%s, FileName=%s, MD5=%s", task.TaskID, task.FileName, task.FileMD5)

			// 执行带指数退避的有限次重试，防止网络瞬断导致任务永久失败
			const maxRetries = 3
			var procErr error
			for attempt := 1; attempt <= maxRetries; attempt++ {
				procErr = c.processTask(ctx, &task)
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

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				g.Log().Warningf(ctx, "[Kafka Consumer] 提交消息 Offset 失败: %v", err)
			}
		}
	}
}

func (c *IndexingConsumer) processTask(ctx context.Context, task *client.FileProcessingTask) error {
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

	// 4. 若为富媒体文档 (PDF, Word, Excel, PPT, HTML 等)，通过 Apache Tika 提取纯文本
	indexTargetFilePath := localPath
	ext := strings.ToLower(filepath.Ext(task.FileName))
	richTextExts := map[string]bool{
		".pdf": true, ".docx": true, ".doc": true, ".xlsx": true, ".xls": true, ".pptx": true, ".ppt": true, ".html": true, ".htm": true,
	}

	if richTextExts[ext] {
		g.Log().Infof(ctx, "[Kafka Consumer] 检测到富媒体文档 %s，使用 Apache Tika 提取正文...", task.FileName)
		f, openErr := os.Open(localPath)
		if openErr != nil {
			c.markFailed(ctx, taskKey, fmt.Sprintf("打开本地文件失败: %v", openErr))
			return openErr
		}
		defer f.Close()

		extractedText, extractErr := c.tikaClient.ExtractText(ctx, f, task.FileName)
		if extractErr != nil {
			g.Log().Warningf(ctx, "[Kafka Consumer] Tika 提取文本失败: %v，降级为直接索引", extractErr)
		} else if strings.TrimSpace(extractedText) != "" {
			convertedPath := localPath + ".txt"
			if writeErr := gfile.PutContents(convertedPath, extractedText); writeErr == nil {
				indexTargetFilePath = convertedPath
				g.Log().Infof(ctx, "[Kafka Consumer] Tika 成功提取正文 (%d 字符)，保存为 %s", len(extractedText), convertedPath)
			}
		}
	}

	// 5. 调用向量化 Pipeline 构建知识库
	ids, err := knowledge_index_pipeline.IndexDocument(ctx, indexTargetFilePath)
	if err != nil {
		c.markFailed(ctx, taskKey, fmt.Sprintf("构建向量索引失败: %v", err))
		return err
	}

	// 6. 更新任务状态为 COMPLETED
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

// Close 停止消费并释放相关资源
func (c *IndexingConsumer) Close() error {
	if c.reader != nil {
		return c.reader.Close()
	}
	return nil
}

// InitAndStartConsumer 根据配置快速初始化并启动后台消费监听
func InitAndStartConsumer(ctx context.Context) (*IndexingConsumer, error) {
	// Redis
	rdb := client.GetRedisClient()

	// MinIO
	minioCfg := config.C.MinIO
	ms, err := storage.NewMinioStorage(minioCfg.Endpoint, minioCfg.AccessKey, minioCfg.SecretKey, minioCfg.Bucket, minioCfg.UseSSL)
	if err != nil {
		return nil, fmt.Errorf("init minio failed: %w", err)
	}

	// Kafka Reader
	reader := client.NewKafkaReader(config.C.Kafka.Brokers, config.C.Kafka.Topic, config.C.Kafka.GroupID)

	// Tika
	tikaClient := client.NewTikaClient(config.C.Tika.ServerUrl)

	indexingConsumer := NewIndexingConsumer(reader, ms, rdb, tikaClient)

	go indexingConsumer.Start(ctx)

	return indexingConsumer, nil
}
