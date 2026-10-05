package models

import (
	"SuperBizAgent/internal/config"
	"context"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

func OpenAIForDeepSeekV31Think(ctx context.Context) (cm model.ToolCallingChatModel, err error) {
	conf := config.C.DsThinkChatModel
	chatConfig := &openai.ChatModelConfig{
		Model:   conf.Model,
		APIKey:  conf.ApiKey,
		BaseURL: conf.BaseUrl,
	}
	return openai.NewChatModel(ctx, chatConfig)
}

func OpenAIForDeepSeekV3Quick(ctx context.Context) (cm model.ToolCallingChatModel, err error) {
	conf := config.C.DsQuickChatModel
	chatConfig := &openai.ChatModelConfig{
		Model:   conf.Model,
		APIKey:  conf.ApiKey,
		BaseURL: conf.BaseUrl,
	}
	return openai.NewChatModel(ctx, chatConfig)
}
