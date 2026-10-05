package plan_execute_replan

import (
	"SuperBizAgent/internal/ai/models"
	"SuperBizAgent/internal/ai/tools"
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

func NewExecutor(ctx context.Context) (adk.Agent, error) {
	// 1. 核心运维排障工具链 (Sentry 异常检索与堆栈诊断 + Prometheus 告警 + 内部知识库 + 系统时间)
	toolList := []tool.BaseTool{
		tools.NewQuerySentryIssuesTool(),
		tools.NewGetSentryIssueDetailTool(),
		tools.NewPrometheusAlertsQueryTool(),
		tools.NewQueryInternalDocsTool(),
		tools.NewGetCurrentTimeTool(),
	}

	// 2. 若配置了外部 MCP Server，尝试动态加载 MCP 工具（支持连接失败优雅降级）
	if mcpTools, err := tools.GetLogMcpTool(); err == nil && len(mcpTools) > 0 {
		toolList = append(toolList, mcpTools...)
	}

	execModel, err := models.OpenAIForDeepSeekV3Quick(ctx)
	if err != nil {
		return nil, err
	}
	return planexecute.NewExecutor(ctx, &planexecute.ExecutorConfig{
		Model: execModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: toolList,
			},
		},
		MaxIterations: 999999,
	})
}
