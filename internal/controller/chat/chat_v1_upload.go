package chat

import (
	"context"
	"fmt"
	"time"

	"SuperBizAgent/api/chat/v1"
	"SuperBizAgent/pkg/kafka"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// UploadCheck 检查文件秒传状态与分片上传进度
func (c *ControllerV1) UploadCheck(ctx context.Context, req *v1.UploadCheckReq) (res *v1.UploadCheckRes, err error) {
	if c.minioStorage == nil || c.bitmapManager == nil {
		return nil, gerror.New("存储与状态服务未就绪")
	}

	mergedKey := c.minioStorage.MergedPath(req.FileMD5, req.FileName)
	// 1. 检查 MinIO 中是否已存在合并后的最终文件（秒传判定）
	if c.minioStorage.Exists(ctx, mergedKey) {
		return &v1.UploadCheckRes{
			IsUploaded:     true,
			UploadedChunks: []int{},
			FileURL:        mergedKey,
		}, nil
	}

	// 2. 从 Redis Bitmap 查询已上传的分片索引
	uploadedChunks, err := c.bitmapManager.GetUploadedChunks(ctx, req.FileMD5, req.TotalChunks)
	if err != nil {
		return nil, gerror.Wrapf(err, "获取分片上传进度失败")
	}

	return &v1.UploadCheckRes{
		IsUploaded:     false,
		UploadedChunks: uploadedChunks,
	}, nil
}

// UploadChunk 上传单个分片到 MinIO 并更新 Redis Bitmap 状态
func (c *ControllerV1) UploadChunk(ctx context.Context, req *v1.UploadChunkReq) (res *v1.UploadChunkRes, err error) {
	if c.minioStorage == nil || c.bitmapManager == nil {
		return nil, gerror.New("存储与状态服务未就绪")
	}

	r := g.RequestFromCtx(ctx)
	uploadFile := r.GetUploadFile("file")
	if uploadFile == nil {
		return nil, gerror.New("分片文件数据不能为空")
	}

	src, err := uploadFile.Open()
	if err != nil {
		return nil, gerror.Wrapf(err, "读取分片数据失败")
	}
	defer src.Close()

	// 1. 将分片以流式写入 MinIO 暂存路径 chunks/{fileMD5}/{chunkIndex}
	if err := c.minioStorage.PutChunk(ctx, req.FileMD5, req.ChunkIndex, src, uploadFile.Size); err != nil {
		return nil, gerror.Wrapf(err, "保存分片到对象存储失败")
	}

	// 2. 将分片索引标记到 Redis Bitmap
	if err := c.bitmapManager.MarkChunk(ctx, req.FileMD5, req.ChunkIndex); err != nil {
		return nil, gerror.Wrapf(err, "更新分片状态失败")
	}

	return &v1.UploadChunkRes{
		ChunkIndex: req.ChunkIndex,
		Success:    true,
	}, nil
}

// UploadMerge 校验所有分片完整性，执行 MinIO 按照 MD5 合并分片，并异步投递 Kafka 任务
func (c *ControllerV1) UploadMerge(ctx context.Context, req *v1.UploadMergeReq) (res *v1.UploadMergeRes, err error) {
	if c.minioStorage == nil || c.bitmapManager == nil || c.kafkaProducer == nil {
		return nil, gerror.New("服务组件未就绪")
	}

	// 1. 从 Redis Bitmap 校验所有分片是否全部上传完毕
	if !c.bitmapManager.IsAllUploaded(ctx, req.FileMD5, req.TotalChunks) {
		return nil, gerror.New("分片未全部上传完成，无法执行合并")
	}

	// 2. 调用 MinIO 按 MD5 进行分片合并
	destKey, err := c.minioStorage.ComposeChunks(ctx, req.FileMD5, req.FileName, req.TotalChunks)
	if err != nil {
		return nil, gerror.Wrapf(err, "MinIO 分片合并失败")
	}

	// 3. 异步清理分片临时对象与 Redis Bitmap，避免阻塞请求
	go func(md5 string, total int) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_ = c.minioStorage.ClearChunks(bgCtx, md5, total)
		_ = c.bitmapManager.Clear(bgCtx, md5)
	}(req.FileMD5, req.TotalChunks)

	// 4. 组装异步任务载荷并投递至 Kafka
	taskID := fmt.Sprintf("task_%s_%d", req.FileMD5, time.Now().UnixNano())
	task := kafka.FileProcessingTask{
		TaskID:         taskID,
		FileMD5:        req.FileMD5,
		FileName:       req.FileName,
		MinIOBucket:    c.minioStorage.BucketName(),
		MinIOObjectKey: destKey,
		TotalSize:      req.TotalSize,
		CreatedAt:      time.Now().UnixMilli(),
	}

	if err := c.kafkaProducer.SendTask(ctx, task); err != nil {
		return nil, gerror.Wrapf(err, "投递 Kafka 向量化任务失败")
	}

	return &v1.UploadMergeRes{
		TaskID:         taskID,
		MinIOObjectKey: destKey,
		Status:         "PENDING",
	}, nil
}
