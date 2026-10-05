package memory

import (
	"context"
	"fmt"
	"strings"

	"SuperBizAgent/internal/ai/models"
	"SuperBizAgent/internal/model"

	einoModel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Summarizer 负责利用轻量 LLM 提取增量会话摘要
type Summarizer struct {
	cm einoModel.ToolCallingChatModel
}

// NewSummarizer 初始化摘要生成器
func NewSummarizer(ctx context.Context) (*Summarizer, error) {
	cm, err := models.OpenAIForDeepSeekV3Quick(ctx)
	if err != nil {
		return nil, fmt.Errorf("init quick chat model for summarizer failed: %w", err)
	}
	return &Summarizer{cm: cm}, nil
}

// Summarize 结合已有摘要与最新对话，提炼出紧凑累进摘要
func (s *Summarizer) Summarize(ctx context.Context, existingSummary string, msgs []*model.Message) (string, error) {
	if len(msgs) == 0 {
		return existingSummary, nil
	}

	var sb strings.Builder
	for _, m := range msgs {
		sb.WriteString(fmt.Sprintf("%s: %s\n", m.Role, m.Content))
	}

	prompt := fmt.Sprintf(`请结合已有背景摘要（若有）与最新发生的对话，提炼生成一段简明扼要的会话累进摘要（控制在 150 字以内）。
要求：
1. 保留关键故障现象、根因结论、已执行的排障操作与重要实体参数；
2. 剔除无意义的寒暄客套，直奔核心主题；
3. 输出为一段纯文本，不要带有 Markdown 标题或列表。

【已有背景摘要】：
%s

【最新多轮对话】：
%s
`, existingSummary, sb.String())

	messages := []*schema.Message{
		schema.SystemMessage("你是一个企业级系统运维专家和上下文压缩助手，擅长在极短篇幅内提炼对话的核心事实与行动结果。"),
		schema.UserMessage(prompt),
	}

	resp, err := s.cm.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("call llm to generate summary failed: %w", err)
	}
	if resp == nil || resp.Content == "" {
		return existingSummary, nil
	}

	return strings.TrimSpace(resp.Content), nil
}
