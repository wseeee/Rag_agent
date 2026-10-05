package model

import (
	"context"
	"fmt"
	"time"

	"SuperBizAgent/pkg/client"

	"gorm.io/gorm"
)

// KnowledgeDocument 知识库文档实体
type KnowledgeDocument struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	FileMD5     string    `gorm:"column:file_md5;type:varchar(32);uniqueIndex" json:"file_md5"`
	FileName    string    `gorm:"column:file_name;type:varchar(255)" json:"file_name"`
	FileSize    int64     `gorm:"column:file_size;default:0" json:"file_size"`
	FileExt     string    `gorm:"column:file_ext;type:varchar(16)" json:"file_ext"`
	MinioBucket string    `gorm:"column:minio_bucket;type:varchar(64)" json:"minio_bucket"`
	MinioKey    string    `gorm:"column:minio_key;type:varchar(512)" json:"minio_key"`
	Status      string    `gorm:"column:status;type:varchar(32);default:UPLOADED" json:"status"` // UPLOADED, PARSING, INDEXING, COMPLETED, FAILED
	ChunkCount  int       `gorm:"column:chunk_count;default:0" json:"chunk_count"`
	ErrorMsg    string    `gorm:"column:error_msg;type:text" json:"error_msg,omitempty"`
	UserID      string    `gorm:"column:user_id;type:varchar(64);default:default_user" json:"user_id"`
	IsDeleted   bool      `gorm:"column:is_deleted;default:false" json:"is_deleted"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (KnowledgeDocument) TableName() string {
	return "knowledge_document"
}

// DocumentDAO 封装知识库文档相关的数据库操作
type DocumentDAO struct{}

var DocumentModel = &DocumentDAO{}

func (d *DocumentDAO) getDB(ctx context.Context) *gorm.DB {
	return client.GetDB().WithContext(ctx)
}

// Upsert 创建或更新知识库文档记录
func (d *DocumentDAO) Upsert(ctx context.Context, doc *KnowledgeDocument) error {
	if doc == nil || doc.FileMD5 == "" {
		return fmt.Errorf("document or file md5 cannot be empty")
	}

	var existing KnowledgeDocument
	err := d.getDB(ctx).Where("file_md5 = ?", doc.FileMD5).First(&existing).Error
	if err == nil {
		doc.ID = existing.ID
		return d.getDB(ctx).Save(doc).Error
	}
	if err == gorm.ErrRecordNotFound {
		return d.getDB(ctx).Create(doc).Error
	}
	return err
}

// GetByMD5 根据 MD5 查询文档详情
func (d *DocumentDAO) GetByMD5(ctx context.Context, fileMD5 string) (*KnowledgeDocument, error) {
	var doc KnowledgeDocument
	err := d.getDB(ctx).Where("file_md5 = ? AND is_deleted = 0", fileMD5).First(&doc).Error
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// UpdateStatus 更新文档的处理状态
func (d *DocumentDAO) UpdateStatus(ctx context.Context, fileMD5, status, errorMsg string, chunkCount int) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if errorMsg != "" {
		updates["error_msg"] = errorMsg
	}
	if chunkCount > 0 {
		updates["chunk_count"] = chunkCount
	}
	return d.getDB(ctx).Model(&KnowledgeDocument{}).
		Where("file_md5 = ?", fileMD5).
		Updates(updates).Error
}

// List 分页查询知识库文档列表
func (d *DocumentDAO) List(ctx context.Context, userID string, page, size int) ([]*KnowledgeDocument, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	query := d.getDB(ctx).Model(&KnowledgeDocument{}).Where("is_deleted = 0")
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []*KnowledgeDocument
	offset := (page - 1) * size
	err := query.Order("created_at DESC").
		Offset(offset).
		Limit(size).
		Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// Delete 软删除文档
func (d *DocumentDAO) Delete(ctx context.Context, fileMD5 string) error {
	return d.getDB(ctx).Model(&KnowledgeDocument{}).
		Where("file_md5 = ?", fileMD5).
		Update("is_deleted", true).Error
}
