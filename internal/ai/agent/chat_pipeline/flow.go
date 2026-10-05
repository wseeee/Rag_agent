package chat_pipeline

import (
	"SuperBizAgent/internal/ai/tools"
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
)

func newReactAgentLambda(ctx context.Context) (lba *compose.Lambda, err error) {
	config := &react.AgentConfig{
		MaxStep:            25,
		ToolReturnDirectly: map[string]struct{}{},
	}
	chatModelIns11, err := newChatModel(ctx)
	if err != nil {
		return nil, err
	}
	config.ToolCallingModel = chatModelIns11
	//searchTool, err := newSearchTool(ctx)
	//if err != nil {
	//	return nil, err
	//}
	// 动态加载工具链：若外部 MCP Server 可用则载入，不可用时优雅降级
	toolList := make([]tool.BaseTool, 0, 8)
	if mcpTool, err := tools.GetLogMcpTool(); err == nil && len(mcpTool) > 0 {
		toolList = append(toolList, mcpTool...)
	}

	toolList = append(toolList,
		tools.NewPrometheusAlertsQueryTool(),
		tools.NewMysqlCrudTool(),
		tools.NewGetCurrentTimeTool(),
		tools.NewQueryInternalDocsTool(),
		tools.NewQuerySentryIssuesTool(),
		tools.NewGetSentryIssueDetailTool(),
	)
	config.ToolsConfig.Tools = toolList

	ins, err := react.NewAgent(ctx, config)
	if err != nil {
		return nil, err
	}
	lba, err = compose.AnyLambda(ins.Generate, ins.Stream, nil, nil)
	if err != nil {
		return nil, err
	}
	return lba, nil
}
