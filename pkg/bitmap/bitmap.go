package bitmap

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ChunkBitmap 封装 Redis BitMap 实现高效的分片上传进度与断点记录
type ChunkBitmap struct {
	client *redis.Client
	ttl    time.Duration
}

// NewChunkBitmap 创建分片位图管理器，默认 24 小时自动过期防止废弃切片残留
func NewChunkBitmap(client *redis.Client) *ChunkBitmap {
	return &ChunkBitmap{
		client: client,
		ttl:    24 * time.Hour,
	}
}

func (b *ChunkBitmap) key(fileMD5 string) string {
	return fmt.Sprintf("upload:chunks:%s", fileMD5)
}

// MarkChunk 将指定分片序号在位图中标记为 1，并刷新 TTL
func (b *ChunkBitmap) MarkChunk(ctx context.Context, fileMD5 string, chunkIndex int) error {
	k := b.key(fileMD5)
	pipe := b.client.Pipeline()
	pipe.SetBit(ctx, k, int64(chunkIndex), 1)
	pipe.Expire(ctx, k, b.ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// GetUploadedChunks 解析位图字节流，返回所有已完成上传的分片索引切片
func (b *ChunkBitmap) GetUploadedChunks(ctx context.Context, fileMD5 string, totalChunks int) ([]int, error) {
	k := b.key(fileMD5)
	data, err := b.client.Get(ctx, k).Bytes()
	if err != nil {
		if err == redis.Nil {
			return []int{}, nil
		}
		return nil, err
	}

	uploaded := make([]int, 0, totalChunks)
	for i := 0; i < totalChunks; i++ {
		byteIndex := i / 8
		bitIndex := i % 8
		if byteIndex < len(data) && (data[byteIndex]>>(7-bitIndex))&1 == 1 {
			uploaded = append(uploaded, i)
		}
	}
	return uploaded, nil
}

// IsAllUploaded 检查是否所有分片（从 0 到 totalChunks-1）全部上传完毕
func (b *ChunkBitmap) IsAllUploaded(ctx context.Context, fileMD5 string, totalChunks int) bool {
	if totalChunks <= 0 {
		return false
	}
	list, err := b.GetUploadedChunks(ctx, fileMD5, totalChunks)
	if err != nil || len(list) != totalChunks {
		return false
	}
	return true
}

// Clear 清理 Redis 中的分片位图标记
func (b *ChunkBitmap) Clear(ctx context.Context, fileMD5 string) error {
	return b.client.Del(ctx, b.key(fileMD5)).Err()
}
