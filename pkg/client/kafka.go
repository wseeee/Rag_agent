package client

import (
	"time"

	"github.com/segmentio/kafka-go"
)

// FileProcessingTask 定义在 Kafka 中传输的文件异步向量化任务载荷
type FileProcessingTask struct {
	TaskID         string `json:"task_id"`
	FileMD5        string `json:"file_md5"`
	FileName       string `json:"file_name"`
	MinIOBucket    string `json:"minio_bucket"`
	MinIOObjectKey string `json:"minio_object_key"`
	TotalSize      int64  `json:"total_size"`
	CreatedAt      int64  `json:"created_at"`
}

// NewKafkaWriter 初始化并返回指定 broker 与 topic 的 Kafka 生产者（Writer）。
func NewKafkaWriter(brokers []string, topic string) *kafka.Writer {
	return &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		Balancer:               &kafka.Hash{},
		WriteTimeout:           5 * time.Second,
		RequiredAcks:           kafka.RequireOne,
		AllowAutoTopicCreation: true,
	}
}

// NewKafkaReader 初始化并返回指定消费者组与 topic 的 Kafka 消费者（Reader）。
func NewKafkaReader(brokers []string, topic, groupID string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		CommitInterval: time.Second,
		StartOffset:    kafka.FirstOffset,
	})
}
