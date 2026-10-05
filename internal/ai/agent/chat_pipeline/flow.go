package chat_pipeline

import (
	"SuperBizAgent/internal/ai/tools"
	"context"
	"io"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

func newReactAgentLambda(ctx context.Context) (lba *compose.Lambda, err error) {
	config := &react.AgentConfig{
		MaxStep:            25,
		ToolReturnDirectly: map[string]struct{}{},
		StreamToolCallChecker: func(ctx context.Context, sr *schema.StreamReader[*schema.Message]) (bool, error) {
			defer sr.Close()
			for {
				msg, err := sr.Recv()
				if err == io.EOF {
					return false, nil
				}
				if err != nil {
					return false, err
				}
				if len(msg.ToolCalls) > 0 {
					return true, nil
				}
			}
		},
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
