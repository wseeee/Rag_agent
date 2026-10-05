package chat

import (
	"SuperBizAgent/api/chat/v1"
	"SuperBizAgent/internal/ai/agent/plan_execute_replan"
	"context"
	"errors"
)

func (c *ControllerV1) AIOps(ctx context.Context, req *v1.AIOpsReq) (res *v1.AIOpsRes, err error) {
	query := `
1. 你是一个智能的服务告警与故障诊断分析助手，首先调用工具 query_prometheus_alerts 获取当前所有活跃的告警。
2. 调用工具 query_sentry_issues 检索 Sentry 平台上报的最新未解决(unresolved)异常错误，若有具体报错，进一步调用 get_sentry_issue_detail 获取调用栈与代码行号。
3. 根据告警名称与 Sentry 报错信息，调用工具 query_internal_docs 获取内部知识库中对应的标准排障方案与 SOP。
4. 涉及到时间的参数都需要先通过工具 get_current_time 获取当前时间，再结合时间要求进行传参。
5. 综合活跃告警、Sentry 异常堆栈分析、内部知识库文档，进行根因推断，生成结构化告警运维分析报告，格式如下：
告警分析报告
---
# 告警处理详情
## 活跃告警清单
## Sentry 异常堆栈排查
## 告警根因分析N(第N个告警)
## 处理方案执行N(第N个告警)
## 结论与行动建议
`

	resp, detail, err := plan_execute_replan.BuildPlanAgent(ctx, query)
	if err != nil {
		return nil, err
	}
	if resp == "" {
		return nil, errors.New("内部错误")
	}
	res = &v1.AIOpsRes{
		Result: resp,
		Detail: detail,
	}
	return res, nil

}
