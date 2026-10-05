package model

import (
	"context"
	"fmt"
	"time"

	"SuperBizAgent/pkg/client"

	"gorm.io/gorm"
)

// Conversation 会话主表实体
type Conversation struct {
	ID            string    `gorm:"primaryKey;column:id;type:varchar(64)" json:"id"`
	UserID        string    `gorm:"column:user_id;type:varchar(64);default:default_user" json:"user_id"`
	Title         string    `gorm:"column:title;type:varchar(255);default:新对话" json:"title"`
	Summary       string    `gorm:"column:summary;type:text" json:"summary"`
	MessageCount  int       `gorm:"column:message_count;default:0" json:"message_count"`
	IsPinned      bool      `gorm:"column:is_pinned;default:false" json:"is_pinned"`
	IsDeleted     bool      `gorm:"column:is_deleted;default:false" json:"is_deleted"`
	LastMessageAt time.Time `gorm:"column:last_message_at;default:CURRENT_TIMESTAMP" json:"last_message_at"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (Conversation) TableName() string {
	return "conversation"
}

// ConversationDAO 封装会话相关的数据库操作
type ConversationDAO struct{}

var ConversationModel = &ConversationDAO{}

func (d *ConversationDAO) getDB(ctx context.Context) *gorm.DB {
	return client.GetDB().WithContext(ctx)
}

// GetOrCreate 获取会话，若不存在则创建
func (d *ConversationDAO) GetOrCreate(ctx context.Context, convID, userID, title string) (*Conversation, error) {
	if convID == "" {
		return nil, fmt.Errorf("conversation id cannot be empty")
	}
	if userID == "" {
		userID = "default_user"
	}
	if title == "" {
		title = "新对话"
	}

	var conv Conversation
	err := d.getDB(ctx).Where("id = ? AND is_deleted = 0", convID).First(&conv).Error
	if err == nil {
		return &conv, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	now := time.Now()
	newConv := Conversation{
		ID:            convID,
		UserID:        userID,
		Title:         title,
		MessageCount:  0,
		IsPinned:      false,
		IsDeleted:     false,
		LastMessageAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := d.getDB(ctx).Create(&newConv).Error; err != nil {
		if err2 := d.getDB(ctx).Where("id = ? AND is_deleted = 0", convID).First(&conv).Error; err2 == nil {
			return &conv, nil
		}
		return nil, err
	}
	return &newConv, nil
}

// GetByID 根据 ID 查询未删除会话
func (d *ConversationDAO) GetByID(ctx context.Context, convID string) (*Conversation, error) {
	var conv Conversation
	err := d.getDB(ctx).Where("id = ? AND is_deleted = 0", convID).First(&conv).Error
	if err != nil {
		return nil, err
	}
	return &conv, nil
}

// List 分页查询用户会话列表（置顶优先，其次按最后消息时间倒序）
func (d *ConversationDAO) List(ctx context.Context, userID string, page, size int) ([]*Conversation, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if userID == "" {
		userID = "default_user"
	}

	var total int64
	query := d.getDB(ctx).Model(&Conversation{}).Where("user_id = ? AND is_deleted = 0", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []*Conversation
	offset := (page - 1) * size
	err := query.Order("is_pinned DESC, last_message_at DESC").
		Offset(offset).
		Limit(size).
		Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// UpdateSummary 更新阶段累进摘要
func (d *ConversationDAO) UpdateSummary(ctx context.Context, convID, summary string) error {
	return d.getDB(ctx).Model(&Conversation{}).
		Where("id = ? AND is_deleted = 0", convID).
		Update("summary", summary).Error
}

// UpdateTitle 更新标题
func (d *ConversationDAO) UpdateTitle(ctx context.Context, convID, title string) error {
	return d.getDB(ctx).Model(&Conversation{}).
		Where("id = ? AND is_deleted = 0", convID).
		Update("title", title).Error
}

// Pin 设置或取消置顶
func (d *ConversationDAO) Pin(ctx context.Context, convID string, isPinned bool) error {
	return d.getDB(ctx).Model(&Conversation{}).
		Where("id = ? AND is_deleted = 0", convID).
		Update("is_pinned", isPinned).Error
}

// Delete 软删除会话
func (d *ConversationDAO) Delete(ctx context.Context, convID string) error {
	return d.getDB(ctx).Model(&Conversation{}).
		Where("id = ?", convID).
		Update("is_deleted", true).Error
}
