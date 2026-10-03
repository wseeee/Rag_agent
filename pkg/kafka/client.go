package kafka

import (
	"context"
	"encoding/json"
	"fmt"
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

// TaskProducer 负责将文档处理任务投递到 Kafka
type TaskProducer struct {
	writer *kafka.Writer
	topic  string
}

// NewTaskProducer 创建一个 Kafka 任务生产者
func NewTaskProducer(brokers []string, topic string) *TaskProducer {
	return &TaskProducer{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			Topic:                  topic,
			Balancer:               &kafka.Hash{},
			WriteTimeout:           5 * time.Second,
			RequiredAcks:           kafka.RequireOne,
			AllowAutoTopicCreation: true,
		},
		topic: topic,
	}
}

// SendTask 发送任务消息到指定 Topic，Key 为 FileMD5 保障相同文件有序处理
func (p *TaskProducer) SendTask(ctx context.Context, task FileProcessingTask) error {
	b, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal task payload failed: %w", err)
	}

	var writeErr error
	for attempt := 0; attempt < 5; attempt++ {
		writeErr = p.writer.WriteMessages(ctx, kafka.Message{
			Key:   []byte(task.FileMD5),
			Value: b,
			Time:  time.Now(),
		})
		if writeErr == nil {
			return nil
		}
		// 若因自动创建 Topic 导致 Leader 正在选举，短暂回退并重试
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
	return writeErr
}

// Close 优雅关闭 Kafka 写入器
func (p *TaskProducer) Close() error {
	return p.writer.Close()
}

// TaskConsumer 负责从 Kafka 消费文档处理任务
type TaskConsumer struct {
	reader *kafka.Reader
}

// NewTaskConsumer 创建一个 Kafka 任务消费者
func NewTaskConsumer(brokers []string, topic, groupID string) *TaskConsumer {
	return &TaskConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:        brokers,
			Topic:          topic,
			GroupID:        groupID,
			MinBytes:       10e3, // 10KB
			MaxBytes:       10e6, // 10MB
			CommitInterval: time.Second,
			StartOffset:    kafka.FirstOffset,
		}),
	}
}

// FetchTask 获取单条任务消息并反序列化，返回任务实体与原始消息以便后续提交 Offset
func (c *TaskConsumer) FetchTask(ctx context.Context) (*FileProcessingTask, *kafka.Message, error) {
	msg, err := c.reader.FetchMessage(ctx)
	if err != nil {
		return nil, nil, err
	}

	var task FileProcessingTask
	if err := json.Unmarshal(msg.Value, &task); err != nil {
		return nil, &msg, fmt.Errorf("unmarshal task payload failed: %w", err)
	}

	return &task, &msg, nil
}

// Commit 提交消息 offset
func (c *TaskConsumer) Commit(ctx context.Context, msg kafka.Message) error {
	return c.reader.CommitMessages(ctx, msg)
}

// Close 关闭 Kafka 消费端
func (c *TaskConsumer) Close() error {
	return c.reader.Close()
}

