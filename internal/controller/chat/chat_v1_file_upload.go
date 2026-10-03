package chat

import (
	"context"
	"os"
	"path/filepath"

	"SuperBizAgent/api/chat/v1"
	"SuperBizAgent/internal/ai/agent/knowledge_index_pipeline"
	"SuperBizAgent/internal/config"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
)

func (c *ControllerV1) FileUpload(ctx context.Context, req *v1.FileUploadReq) (res *v1.FileUploadRes, err error) {
	// 从请求中获取上传的文件
	r := g.RequestFromCtx(ctx)
	uploadFile := r.GetUploadFile("file")
	if uploadFile == nil {
		return nil, gerror.New("请上传文件")
	}

	// 确保保存目录存在
	if !gfile.Exists(config.FileDir) {
		if err := gfile.Mkdir(config.FileDir); err != nil {
			return nil, gerror.Wrapf(err, "创建目录失败: %s", config.FileDir)
		}
	}

	// 获取原始文件名
	newFileName := uploadFile.Filename
	// 完整的保存路径
	savePath := filepath.Join(config.FileDir)

	// 保存文件
	_, err = uploadFile.Save(savePath, false)
	if err != nil {
		return nil, gerror.Wrapf(err, "保存文件失败")
	}

	// 获取文件信息
	fileInfo, err := os.Stat(savePath)
	if err != nil {
		return nil, gerror.Wrapf(err, "获取文件信息失败")
	}

	res = &v1.FileUploadRes{
		FileName: newFileName,
		FilePath: savePath,
		FileSize: fileInfo.Size(),
	}
	_, err = knowledge_index_pipeline.IndexDocument(ctx, config.FileDir+"/"+newFileName)
	if err != nil {
		return nil, gerror.Wrapf(err, "构建知识库失败")
	}
	return res, nil
}

