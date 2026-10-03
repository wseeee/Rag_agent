package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

type ChatReq struct {
	g.Meta   `path:"/chat" method:"post" summary:"对话"`
	Id       string
	Question string
}

type ChatRes struct {
	Answer string `json:"answer"`
}

type ChatStreamReq struct {
	g.Meta   `path:"/chat_stream" method:"post" summary:"流式对话"`
	Id       string
	Question string
}

type ChatStreamRes struct {
}

type AIOpsReq struct {
	g.Meta `path:"/ai_ops" method:"post" summary:"AI运维"`
}

type AIOpsRes struct {
	Result string   `json:"result"`
	Detail []string `json:"detail"`
}

type UploadCheckReq struct {
	g.Meta      `path:"/upload/check" method:"post" summary:"检查分片上传状态与秒传"`
	FileMD5     string `json:"fileMd5" v:"required#文件MD5不能为空"`
	FileName    string `json:"fileName" v:"required#文件名不能为空"`
	TotalChunks int    `json:"totalChunks" v:"required|min:1#分片总数必须大于0"`
}

type UploadCheckRes struct {
	IsUploaded     bool   `json:"isUploaded" dc:"是否秒传（文件已存在且已合并）"`
	UploadedChunks []int  `json:"uploadedChunks" dc:"已上传的分片索引列表"`
	FileURL        string `json:"fileUrl,omitempty" dc:"秒传成功时的文件路径"`
}

type UploadChunkReq struct {
	g.Meta      `path:"/upload/chunk" method:"post" mime:"multipart/form-data" summary:"上传单个分片"`
	FileMD5     string `p:"fileMd5" v:"required#文件MD5不能为空"`
	ChunkIndex  int    `p:"chunkIndex" v:"min:0#分片索引必须非负"`
	TotalChunks int    `p:"totalChunks" v:"required|min:1#分片总数必须大于0"`
}

type UploadChunkRes struct {
	ChunkIndex int  `json:"chunkIndex" dc:"成功上传的分片索引"`
	Success    bool `json:"success" dc:"是否上传成功"`
}

type UploadMergeReq struct {
	g.Meta      `path:"/upload/merge" method:"post" summary:"合并分片并投递处理任务"`
	FileMD5     string `json:"fileMd5" v:"required#文件MD5不能为空"`
	FileName    string `json:"fileName" v:"required#文件名不能为空"`
	TotalChunks int    `json:"totalChunks" v:"required|min:1#分片总数必须大于0"`
	TotalSize   int64  `json:"totalSize" v:"required|min:1#文件总大小必须大于0"`
}

type UploadMergeRes struct {
	TaskID         string `json:"taskId" dc:"异步处理任务ID"`
	MinIOObjectKey string `json:"minioObjectKey" dc:"MinIO对象键"`
	Status         string `json:"status" dc:"任务状态: PENDING/PROCESSING"`
}

// DocumentInfo 表示知识库文档条目信息
type DocumentInfo struct {
	FileMD5      string `json:"fileMd5" dc:"文件MD5"`
	FileName     string `json:"fileName" dc:"文件名"`
	FileSize     int64  `json:"fileSize" dc:"文件字节大小"`
	LastModified string `json:"lastModified" dc:"最后修改时间"`
}

type DocumentListReq struct {
	g.Meta `path:"/documents/list" method:"get" summary:"获取知识库已上传文档列表"`
}

type DocumentListRes struct {
	Total int            `json:"total" dc:"文档总数"`
	Items []DocumentInfo `json:"items" dc:"文档列表"`
}

type DocumentDeleteReq struct {
	g.Meta  `path:"/documents/:fileMd5" method:"delete" summary:"级联删除指定知识库文档"`
	FileMD5 string `p:"fileMd5" v:"required#文件MD5不能为空"`
}

type DocumentDeleteRes struct {
	Success bool   `json:"success" dc:"是否删除成功"`
	Message string `json:"message" dc:"操作提示"`
}

type DocumentDownloadReq struct {
	g.Meta   `path:"/documents/download" method:"get" summary:"获取文档临时预签名下载链接"`
	FileMD5  string `p:"fileMd5" v:"required#文件MD5不能为空"`
	FileName string `p:"fileName" v:"required#文件名不能为空"`
}

type DocumentDownloadRes struct {
	FileName    string `json:"fileName" dc:"文件名"`
	DownloadURL string `json:"downloadUrl" dc:"预签名下载链接"`
	ExpiresIn   int64  `json:"expiresIn" dc:"有效秒数"`
}

type DocumentPreviewReq struct {
	g.Meta   `path:"/documents/preview" method:"get" summary:"在线预览文档内容"`
	FileMD5  string `p:"fileMd5" v:"required#文件MD5不能为空"`
	FileName string `p:"fileName" v:"required#文件名不能为空"`
	MaxBytes int64  `p:"maxBytes" d:"16384" dc:"最大读取字节数"`
}

type DocumentPreviewRes struct {
	FileName string `json:"fileName" dc:"文件名"`
	Content  string `json:"content" dc:"文本内容"`
	FileSize int64  `json:"fileSize" dc:"总文件大小"`
}

