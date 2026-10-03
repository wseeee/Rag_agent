package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioStorage 封装 MinIO 对象存储，提供按 MD5 分片暂存、原子合并与生命周期清理
type MinioStorage struct {
	client     *minio.Client
	bucketName string
}

// NewMinioStorage 初始化 MinIO 存储实例并自动确保存储桶存在
func NewMinioStorage(endpoint, accessKey, secretKey, bucketName string, useSSL bool) (*MinioStorage, error) {
	cli, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("init minio client failed: %w", err)
	}

	ctx := context.Background()
	exists, err := cli.BucketExists(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("check bucket exists failed: %w", err)
	}
	if !exists {
		if err := cli.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("make bucket failed: %w", err)
		}
	}

	return &MinioStorage{
		client:     cli,
		bucketName: bucketName,
	}, nil
}

// ChunkPath 返回规范化的分片存储路径: chunks/{fileMD5}/{chunkIndex}
func (s *MinioStorage) ChunkPath(fileMD5 string, chunkIndex int) string {
	return fmt.Sprintf("chunks/%s/%d", fileMD5, chunkIndex)
}

// MergedPath 返回规范化的合并目标路径: merged/{fileMD5}/{cleanFileName}
func (s *MinioStorage) MergedPath(fileMD5, fileName string) string {
	cleanName := filepath.Base(fileName)
	return fmt.Sprintf("merged/%s/%s", fileMD5, cleanName)
}

// PutChunk 流式保存单个分片到 MinIO
func (s *MinioStorage) PutChunk(ctx context.Context, fileMD5 string, chunkIndex int, reader io.Reader, size int64) error {
	objectName := s.ChunkPath(fileMD5, chunkIndex)
	_, err := s.client.PutObject(ctx, s.bucketName, objectName, reader, size, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	return err
}

// ComposeChunks 按照分片序号顺序执行 MinIO 原子合并
func (s *MinioStorage) ComposeChunks(ctx context.Context, fileMD5, fileName string, totalChunks int) (string, error) {
	destKey := s.MergedPath(fileMD5, fileName)

	// 单分片特殊处理：直接使用 CopyObject
	if totalChunks == 1 {
		src := minio.CopySrcOptions{
			Bucket: s.bucketName,
			Object: s.ChunkPath(fileMD5, 0),
		}
		dst := minio.CopyDestOptions{
			Bucket: s.bucketName,
			Object: destKey,
		}
		_, err := s.client.CopyObject(ctx, dst, src)
		return destKey, err
	}

	// 检查第 0 个分片大小以判断是否满足 S3 ComposeObject (>= 5MB) 要求
	part0Info, err := s.client.StatObject(ctx, s.bucketName, s.ChunkPath(fileMD5, 0), minio.StatObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("stat chunk 0 failed: %w", err)
	}

	const minPartSize = 5 * 1024 * 1024 // 5MB
	if part0Info.Size >= minPartSize {
		// 分片 >= 5MB 时使用 S3 原生服务端 ComposeObject (极速合并)
		srcs := make([]minio.CopySrcOptions, totalChunks)
		for i := 0; i < totalChunks; i++ {
			srcs[i] = minio.CopySrcOptions{
				Bucket: s.bucketName,
				Object: s.ChunkPath(fileMD5, i),
			}
		}
		dst := minio.CopyDestOptions{
			Bucket: s.bucketName,
			Object: destKey,
		}
		_, err := s.client.ComposeObject(ctx, dst, srcs...)
		return destKey, err
	}

	// 当切片 < 5MB 时使用 io.MultiReader 流式组合；若切片数 > 1000 时需改为分批拼接或要求前端切片 >= 5MB
	readers := make([]io.Reader, totalChunks)
	objects := make([]*minio.Object, totalChunks)
	defer func() {
		for _, o := range objects {
			if o != nil {
				_ = o.Close()
			}
		}
	}()

	var totalSize int64 = 0
	for i := 0; i < totalChunks; i++ {
		obj, err := s.client.GetObject(ctx, s.bucketName, s.ChunkPath(fileMD5, i), minio.GetObjectOptions{})
		if err != nil {
			return "", fmt.Errorf("get chunk %d failed: %w", i, err)
		}
		objects[i] = obj
		info, err := obj.Stat()
		if err != nil {
			return "", fmt.Errorf("stat chunk %d failed: %w", i, err)
		}
		totalSize += info.Size
		readers[i] = obj
	}

	combinedReader := io.MultiReader(readers...)
	_, err = s.client.PutObject(ctx, s.bucketName, destKey, combinedReader, totalSize, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	return destKey, err
}

// ClearChunks 批量清理已合并的分片临时文件
func (s *MinioStorage) ClearChunks(ctx context.Context, fileMD5 string, totalChunks int) error {
	objectsCh := make(chan minio.ObjectInfo, totalChunks)
	go func() {
		defer close(objectsCh)
		for i := 0; i < totalChunks; i++ {
			objectsCh <- minio.ObjectInfo{Key: s.ChunkPath(fileMD5, i)}
		}
	}()

	for err := range s.client.RemoveObjects(ctx, s.bucketName, objectsCh, minio.RemoveObjectsOptions{}) {
		if err.Err != nil {
			return err.Err
		}
	}
	return nil
}

// GetObject 获取合并后文件流
func (s *MinioStorage) GetObject(ctx context.Context, objectName string) (*minio.Object, error) {
	return s.client.GetObject(ctx, s.bucketName, objectName, minio.GetObjectOptions{})
}

// RemoveObject 删除指定对象
func (s *MinioStorage) RemoveObject(ctx context.Context, objectName string) error {
	return s.client.RemoveObject(ctx, s.bucketName, objectName, minio.RemoveObjectOptions{})
}

// Exists 检查指定对象是否存在于存储桶中
func (s *MinioStorage) Exists(ctx context.Context, objectName string) bool {
	_, err := s.client.StatObject(ctx, s.bucketName, objectName, minio.StatObjectOptions{})
	return err == nil
}

// DownloadObject 下载 MinIO 中的对象到本地文件
func (s *MinioStorage) DownloadObject(ctx context.Context, objectName, localFilePath string) error {
	return s.client.FGetObject(ctx, s.bucketName, objectName, localFilePath, minio.GetObjectOptions{})
}

// BucketName 返回当前存储桶名称
func (s *MinioStorage) BucketName() string {
	return s.bucketName
}


