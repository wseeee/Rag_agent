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
	FileUpload(ctx context.Context, req *v1.FileUploadReq) (res *v1.FileUploadRes, err error)
	AIOps(ctx context.Context, req *v1.AIOpsReq) (res *v1.AIOpsRes, err error)
	UploadCheck(ctx context.Context, req *v1.UploadCheckReq) (res *v1.UploadCheckRes, err error)
	UploadChunk(ctx context.Context, req *v1.UploadChunkReq) (res *v1.UploadChunkRes, err error)
	UploadMerge(ctx context.Context, req *v1.UploadMergeReq) (res *v1.UploadMergeRes, err error)
}
