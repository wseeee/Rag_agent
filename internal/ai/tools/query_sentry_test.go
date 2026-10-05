package tools

import (
	"context"
	"encoding/json"
	"testing"

	"SuperBizAgent/internal/config"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
)

func TestQuerySentryIssuesTool(t *testing.T) {
	ctx := context.Background()

	// 确保加载配置文件
	if adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile); ok {
		_ = adapter.AddPath("etc/config", "etc")
	}
	_ = config.Init(ctx)

	if config.C.Sentry.AuthToken == "" {
		t.Skip("skip sentry test: auth_token is empty")
	}

	tool := NewQuerySentryIssuesTool()
	outStr, err := tool.InvokableRun(ctx, `{"query":"is:unresolved","limit":3}`)
	if err != nil {
		t.Fatalf("InvokableRun failed: %v", err)
	}

	t.Logf("Sentry issues result:\n%s", outStr)

	var res QuerySentryIssuesOutput
	if err := json.Unmarshal([]byte(outStr), &res); err != nil {
		t.Fatalf("unmarshal output failed: %v", err)
	}

	if !res.Success {
		t.Fatalf("expected success, got error: %s", res.Error)
	}

	if len(res.Issues) > 0 {
		firstIssueID := res.Issues[0].ID
		t.Logf("Testing get detail for issue %s (%s)...", firstIssueID, res.Issues[0].ShortID)

		detailTool := NewGetSentryIssueDetailTool()
		detailOutStr, err := detailTool.InvokableRun(ctx, `{"issue_id":"`+firstIssueID+`"}`)
		if err != nil {
			t.Fatalf("detailTool.InvokableRun failed: %v", err)
		}
		t.Logf("Sentry issue detail result:\n%s", detailOutStr)

		var detailRes SentryIssueDetailOutput
		if err := json.Unmarshal([]byte(detailOutStr), &detailRes); err != nil {
			t.Fatalf("unmarshal detail output failed: %v", err)
		}
		if !detailRes.Success {
			t.Fatalf("expected detail success, got error: %s", detailRes.Error)
		}
	}
}

func TestQueryByTraceID(t *testing.T) {
	ctx := context.Background()
	if adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile); ok {
		_ = adapter.AddPath("etc/config", "etc")
	}
	_ = config.Init(ctx)

	if config.C.Sentry.AuthToken == "" {
		t.Skip("skip sentry test: auth_token is empty")
	}

	tool := NewQuerySentryIssuesTool()
	// 测试直接用 trace_id 字段进行精准搜索
	outStr, err := tool.InvokableRun(ctx, `{"trace_id":"2d3eff505543e980f6c11f45f43ccac7"}`)
	if err != nil {
		t.Fatalf("InvokableRun failed: %v", err)
	}
	t.Logf("Search by trace_id result:\n%s", outStr)

	var res QuerySentryIssuesOutput
	if err := json.Unmarshal([]byte(outStr), &res); err != nil {
		t.Fatalf("unmarshal output failed: %v", err)
	}

	if !res.Success || len(res.Issues) == 0 {
		t.Fatalf("expected issues found for trace_id, got %d", len(res.Issues))
	}

	// 用找到的 Issue ID 查详情堆栈
	issueID := res.Issues[0].ID
	detailTool := NewGetSentryIssueDetailTool()
	detailOut, err := detailTool.InvokableRun(ctx, `{"issue_id":"`+issueID+`"}`)
	if err != nil {
		t.Fatalf("detailTool failed: %v", err)
	}
	t.Logf("Issue detail result:\n%s", detailOut)
}
