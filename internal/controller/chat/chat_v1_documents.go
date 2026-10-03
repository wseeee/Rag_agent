package chat

import (
	"context"
	"time"

	v1 "SuperBizAgent/api/chat/v1"
	"SuperBizAgent/internal/ai/agent/knowledge_index_pipeline"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// DocumentList 获取已合并入库的知识库文档列表
func (c *ControllerV1) DocumentList(ctx context.Context, req *v1.DocumentListReq) (res *v1.DocumentListRes, err error) {
	if c.minioStorage == nil {
		return nil, gerror.NewCode(gcode.CodeInternalError, "MinIO 存储服务未就绪")
	}

	items, err := c.minioStorage.ListMergedDocuments(ctx)
	if err != nil {
		g.Log().Errorf(ctx, "list merged documents failed: %v", err)
		return nil, gerror.NewCode(gcode.CodeInternalError, "获取文档列表失败")
	}

	docInfos := make([]v1.DocumentInfo, 0, len(items))
	for _, it := range items {
		docInfos = append(docInfos, v1.DocumentInfo{
			FileMD5:      it.FileMD5,
			FileName:     it.FileName,
			FileSize:     it.Size,
			LastModified: it.LastModified.Format("2006-01-02 15:04:05"),
		})
	}

	return &v1.DocumentListRes{
		Total: len(docInfos),
		Items: docInfos,
	}, nil
}

// DocumentDelete 级联删除指定知识库文档及其所有向量切片索引与 Redis 状态
func (c *ControllerV1) DocumentDelete(ctx context.Context, req *v1.DocumentDeleteReq) (res *v1.DocumentDeleteRes, err error) {
	if c.minioStorage == nil {
		return nil, gerror.NewCode(gcode.CodeInternalError, "MinIO 存储服务未就绪")
	}

	fileMD5 := req.FileMD5
	g.Log().Infof(ctx, "starting cascade deletion for document: %s", fileMD5)

	// 1. 删除 MinIO 物理合并对象及相关分片
	if err := c.minioStorage.DeleteDocumentByMD5(ctx, fileMD5); err != nil {
		g.Log().Warningf(ctx, "delete minio objects for %s warning: %v", fileMD5, err)
	}

	// 2. 级联清理 Milvus 集合中的相关向量索引
	if err := knowledge_index_pipeline.DeleteDocumentVectors(ctx, fileMD5); err != nil {
		g.Log().Warningf(ctx, "delete milvus vectors for %s warning: %v", fileMD5, err)
	}

	// 3. 清理 Redis Bitmap 上传进度与缓存
	if c.bitmapManager != nil {
		_ = c.bitmapManager.Clear(ctx, fileMD5)
	}

	return &v1.DocumentDeleteRes{
		Success: true,
		Message: "文档及关联向量索引已成功级联删除",
	}, nil
}

// DocumentDownload 生成 15 分钟临时预签名下载链接
func (c *ControllerV1) DocumentDownload(ctx context.Context, req *v1.DocumentDownloadReq) (res *v1.DocumentDownloadRes, err error) {
	if c.minioStorage == nil {
		return nil, gerror.NewCode(gcode.CodeInternalError, "MinIO 存储服务未就绪")
	}

	downloadURL, err := c.minioStorage.GetPresignedDownloadURL(ctx, req.FileMD5, req.FileName, 15*time.Minute)
	if err != nil {
		g.Log().Errorf(ctx, "generate presigned download url failed: %v", err)
		return nil, gerror.NewCode(gcode.CodeInternalError, "生成下载链接失败")
	}

	return &v1.DocumentDownloadRes{
		FileName:    req.FileName,
		DownloadURL: downloadURL,
		ExpiresIn:   900,
	}, nil
}

// DocumentPreview 在线预览文档前部文本内容
func (c *ControllerV1) DocumentPreview(ctx context.Context, req *v1.DocumentPreviewReq) (res *v1.DocumentPreviewRes, err error) {
	if c.minioStorage == nil {
		return nil, gerror.NewCode(gcode.CodeInternalError, "MinIO 存储服务未就绪")
	}

	maxBytes := req.MaxBytes
	if maxBytes <= 0 || maxBytes > 65536 {
		maxBytes = 16384
	}

	content, fileSize, err := c.minioStorage.GetDocumentPreview(ctx, req.FileMD5, req.FileName, maxBytes)
	if err != nil {
		g.Log().Errorf(ctx, "get document preview failed: %v", err)
		return nil, gerror.NewCode(gcode.CodeInternalError, "读取文档预览内容失败")
	}

	return &v1.DocumentPreviewRes{
		FileName: req.FileName,
		Content:  content,
		FileSize: fileSize,
	}, nil
}
