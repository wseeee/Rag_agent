package main

import (
	tools2 "SuperBizAgent/internal/ai/tools"
	_ "SuperBizAgent/internal/config"
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

func main() {
	ctx := context.Background()
	// 从 config.yaml 中读取 ChatModel 配置 (优先 volc_chat_model，回退 ds_think_chat_model)
	modelName, _ := g.Cfg().Get(ctx, "volc_chat_model.model")
	apiKey, _ := g.Cfg().Get(ctx, "volc_chat_model.api_key")
	baseURL, _ := g.Cfg().Get(ctx, "volc_chat_model.base_url")

	if apiKey.IsEmpty() {
		modelName, _ = g.Cfg().Get(ctx, "ds_think_chat_model.model")
		apiKey, _ = g.Cfg().Get(ctx, "ds_think_chat_model.api_key")
		baseURL, _ = g.Cfg().Get(ctx, "ds_think_chat_model.base_url")
	}

	config := &openai.ChatModelConfig{
		APIKey:  apiKey.String(),
		Model:   modelName.String(),
		BaseURL: baseURL.String(),
	}
	chatModel, err := openai.NewChatModel(ctx, config)
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
