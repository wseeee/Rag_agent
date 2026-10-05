package main

import (
	"SuperBizAgent/internal/config"
	tools2 "SuperBizAgent/internal/ai/tools"
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	conf := config.C.DsThinkChatModel
	chatModelConfig := &openai.ChatModelConfig{
		APIKey:  conf.ApiKey,
		Model:   conf.Model,
		BaseURL: conf.BaseUrl,
	}
	chatModel, err := openai.NewChatModel(ctx, chatModelConfig)
	if err != nil {
		panic(err)
	}
	// 获取工具信息, 用于绑定到 ChatModel
	toolList, _ := tools2.GetLogMcpTool()
	toolList = append(toolList, tools2.NewGetCurrentTimeTool())
	toolInfos := make([]*schema.ToolInfo, 0)
	var info *schema.ToolInfo
	for _, todoTool := range toolList {
		info, err = todoTool.Info(ctx)
		if err != nil {
			panic(err)
		}
		toolInfos = append(toolInfos, info)
	}

	// 将 tools 绑定到 ChatModel
	err = chatModel.BindTools(toolInfos)
	if err != nil {
		panic(err)
	}

	// 创建一个完整的处理链
	chain := compose.NewChain[[]*schema.Message, *schema.Message]()
	chain.AppendChatModel(chatModel, compose.WithNodeName("chat_model"))

	// 编译并运行 chain
	agent, err := chain.Compile(ctx)
	if err != nil {
		panic(err)
	}
	// 运行示例
	resp, err := agent.Invoke(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "告诉我你有哪些工具可以使用",
		},
	})
	if err != nil {
		panic(err)
	}
	// 输出结果
	fmt.Println(resp.Content)
}
