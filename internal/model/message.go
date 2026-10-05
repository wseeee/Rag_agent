package model

import (
	"context"
	"fmt"
	"time"

	"SuperBizAgent/pkg/client"

	"gorm.io/gorm"
)

// Message 会话消息明细表实体
type Message struct {
	ID               uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	ConversationID   string    `gorm:"column:conversation_id;type:varchar(64);index:idx_conv_created" json:"conversation_id"`
	Role             string    `gorm:"column:role;type:varchar(32)" json:"role"` // user / assistant / system / tool
	Content          string    `gorm:"column:content;type:mediumtext" json:"content"`
	PromptTokens     int       `gorm:"column:prompt_tokens;default:0" json:"prompt_tokens"`
	CompletionTokens int       `gorm:"column:completion_tokens;default:0" json:"completion_tokens"`
	Metadata         *string   `gorm:"column:metadata;type:json" json:"metadata,omitempty"` // JSON string or nil
	IsDeleted        bool      `gorm:"column:is_deleted;default:false" json:"is_deleted"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime;index:idx_conv_created" json:"created_at"`
}

func (Message) TableName() string {
	return "conversation_message"
}

// MessageDAO 封装消息相关的数据库操作
type MessageDAO struct{}

var MessageModel = &MessageDAO{}

func (d *MessageDAO) getDB(ctx context.Context) *gorm.DB {
	return client.GetDB().WithContext(ctx)
}

// SaveInteraction 事务保存一轮用户与助手的问答流水，并更新会话消息总数与最新时间，返回更新后的总消息条数
func (d *MessageDAO) SaveInteraction(
	ctx context.Context,
	convID string,
	userID string,
	userContent string,
	assistantContent string,
	promptTokens int,
	completionTokens int,
	metadata string,
) (int, error) {
	if convID == "" {
		return 0, fmt.Errorf("conversation id cannot be empty")
	}
	if userID == "" {
		userID = "default_user"
	}

	var newCount int
	err := d.getDB(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 确保会话存在
		var conv Conversation
		if err := tx.Where("id = ? AND is_deleted = 0", convID).First(&conv).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				now := time.Now()
				conv = Conversation{
					ID:            convID,
					UserID:        userID,
					Title:         truncateTitle(userContent),
					LastMessageAt: now,
				}
				if err := tx.Create(&conv).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}

		now := time.Now()
		// 2. 插入用户消息
		userMsg := Message{
			ConversationID: convID,
			Role:           "user",
			Content:        userContent,
			PromptTokens:   promptTokens,
			CreatedAt:      now,
		}
		if err := tx.Create(&userMsg).Error; err != nil {
			return err
		}

		var metaPtr *string
		if metadata != "" {
			metaPtr = &metadata
		}

		// 3. 插入助手消息
		asstMsg := Message{
			ConversationID:   convID,
			Role:             "assistant",
			Content:          assistantContent,
			CompletionTokens: completionTokens,
			Metadata:         metaPtr,
			CreatedAt:        now.Add(time.Millisecond),
		}
		if err := tx.Create(&asstMsg).Error; err != nil {
			return err
		}

		// 4. 更新会话状态
		newCount = conv.MessageCount + 2
		updates := map[string]interface{}{
			"message_count":   newCount,
			"last_message_at": now,
		}
		if conv.Title == "新对话" || conv.Title == "" {
			updates["title"] = truncateTitle(userContent)
		}
		return tx.Model(&Conversation{}).Where("id = ?", convID).Updates(updates).Error
	})
	return newCount, err
}

// GetRecentMessages 查询指定会话最近 limit 条未删除消息，按时间升序返回（适合组装 Prompt）
func (d *MessageDAO) GetRecentMessages(ctx context.Context, convID string, limit int) ([]*Message, error) {
	if limit <= 0 {
		limit = 10
	}

	var msgs []*Message
	err := d.getDB(ctx).
		Where("conversation_id = ? AND is_deleted = 0", convID).
		Order("id DESC").
		Limit(limit).
		Find(&msgs).Error
	if err != nil {
		return nil, err
	}

	// 反转切片，恢复时间升序供上下文使用
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// Delete 软删除单条消息
func (d *MessageDAO) Delete(ctx context.Context, msgID uint64) error {
	return d.getDB(ctx).Model(&Message{}).
		Where("id = ?", msgID).
		Update("is_deleted", true).Error
}

func truncateTitle(text string) string {
	r := []rune(text)
	if len(r) == 0 {
		return "新对话"
	}
	if len(r) > 30 {
		return string(r[:30]) + "..."
	}
	return string(r)
}
