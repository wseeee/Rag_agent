package chat_pipeline

import (
	"context"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

type ChatTemplateConfig struct {
	FormatType schema.FormatType
	Templates  []schema.MessagesTemplate
}

// newChatTemplate component initialization function of node 'ChatTemplate' in graph 'EinoAgent'
func newChatTemplate(ctx context.Context) (ctp prompt.ChatTemplate, err error) {
	config := &ChatTemplateConfig{
		FormatType: schema.FString,
		Templates: []schema.MessagesTemplate{
			schema.SystemMessage(systemPrompt),
			schema.MessagesPlaceholder("history", false),
			schema.UserMessage("{content}"),
		},
	}
	ctp = prompt.FromMessages(config.FormatType, config.Templates...)
	return ctp, nil
}

var systemPrompt = `
# 角色：智能运维与知识问答助手

## 核心原则与回答准则
1. **依据知识库作答**：请优先结合下方【相关参考知识库文档】进行解答，回答需先给结论，再给分析与操作指引。
2. **强制出处溯源（Citations）**：凡引用了参考文档中的规则、操作、错误码或论据，请务必在对应句末附带出处标记，格式固定为：(来源#编号: 文件名)，例如：(来源#1: 告警处理手册.md)。
3. **真实性与严谨性**：若参考知识库文档中无足够相关依据，请坦诚告知“当前知识库暂无相关记录”，切勿主观臆测。
4. **输出排版格式**：请使用规范易读的 Markdown 语法（标题、列表、加粗、代码块），便于运维人员阅读。

## 关联运维配置
- 监控日志主题地域：ap-guangzhou；主题ID：869830db-a055-4479-963b-3c898d27e755
- 当前系统时间：{date}

## 相关参考知识库文档
<<REF>>
{documents}
<<END>>
`
