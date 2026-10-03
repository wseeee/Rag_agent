// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package chat

import (
	"context"

	"SuperBizAgent/api/chat/v1"
)

type IChatV1 interface {
	Chat(ctx context.Context, req *v1.ChatReq) (res *v1.ChatRes, err error)
	ChatStream(ctx context.Context, req *v1.ChatStreamReq) (res *v1.ChatStreamRes, err error)
	AIOps(ctx context.Context, req *v1.AIOpsReq) (res *v1.AIOpsRes, err error)
	UploadCheck(ctx context.Context, req *v1.UploadCheckReq) (res *v1.UploadCheckRes, err error)
	UploadChunk(ctx context.Context, req *v1.UploadChunkReq) (res *v1.UploadChunkRes, err error)
	UploadMerge(ctx context.Context, req *v1.UploadMergeReq) (res *v1.UploadMergeRes, err error)
	DocumentList(ctx context.Context, req *v1.DocumentListReq) (res *v1.DocumentListRes, err error)
	DocumentDelete(ctx context.Context, req *v1.DocumentDeleteReq) (res *v1.DocumentDeleteRes, err error)
	DocumentDownload(ctx context.Context, req *v1.DocumentDownloadReq) (res *v1.DocumentDownloadRes, err error)
	DocumentPreview(ctx context.Context, req *v1.DocumentPreviewReq) (res *v1.DocumentPreviewRes, err error)
}
