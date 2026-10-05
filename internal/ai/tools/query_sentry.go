package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"SuperBizAgent/internal/config"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/gogf/gf/v2/frame/g"
)

// QuerySentryIssuesInput 搜索 Sentry 问题的输入
type QuerySentryIssuesInput struct {
	Query   string `json:"query,omitempty" jsonschema:"description=Sentry 检索过滤表达式，例如 'is:unresolved', 或包含错误关键字。默认值为 'is:unresolved'"`
	TraceID string `json:"trace_id,omitempty" jsonschema:"description=分布式链路追踪的 Trace ID (32位十六进制字符串)，如果提供将精准定位该次链路发生的具体报错"`
	Limit   int    `json:"limit,omitempty" jsonschema:"description=返回的最大数量，默认为 5，最大为 20"`
}

// SentryIssueItem 简化的 Issue 概要
type SentryIssueItem struct {
	ID        string `json:"id"`
	ShortID   string `json:"short_id"`
	Title     string `json:"title"`
	Culprit   string `json:"culprit"`
	Level     string `json:"level"`
	Status    string `json:"status"`
	Count     string `json:"count"`
	UserCount int    `json:"user_count"`
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
	Permalink string `json:"permalink"`
}

// QuerySentryIssuesOutput 输出结果
type QuerySentryIssuesOutput struct {
	Success bool               `json:"success"`
	Total   int                `json:"total"`
	Issues  []*SentryIssueItem `json:"issues"`
	Message string             `json:"message,omitempty"`
	Error   string             `json:"error,omitempty"`
}

// GetSentryIssueDetailInput 获取具体 Issue 详情的输入
type GetSentryIssueDetailInput struct {
	IssueID string `json:"issue_id" jsonschema:"description=Sentry Issue 的数字唯一ID，例如 '7773261192'"`
}

// SentryIssueDetailOutput 输出包含堆栈的详细信息
type SentryIssueDetailOutput struct {
	Success    bool     `json:"success"`
	IssueID    string   `json:"issue_id"`
	EventID    string   `json:"event_id,omitempty"`
	TraceID    string   `json:"trace_id,omitempty"`
	Title      string   `json:"title"`
	Message    string   `json:"message"`
	Exception  string   `json:"exception,omitempty"`
	StackTrace []string `json:"stack_trace,omitempty"`
	Permalink  string   `json:"permalink,omitempty"`
	Error      string   `json:"error,omitempty"`
}

func getSentryClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// NewQuerySentryIssuesTool 创建查询 Sentry 报错列表的 Eino 工具
func NewQuerySentryIssuesTool() tool.InvokableTool {
	t, err := utils.InferOptionableTool(
		"query_sentry_issues",
		"查询 Sentry 异常监控平台上报的错误与异常列表。支持传入 trace_id 精准定位特定链路报错原因，或按状态(is:unresolved)、级别(level:error)、关键字过滤。返回 Issue ID、标题、触发文件、报错次数、首次/末次发生时间及直达链接。",
		func(ctx context.Context, input *QuerySentryIssuesInput, opts ...tool.Option) (string, error) {
			cfg := config.C.Sentry
			if cfg.AuthToken == "" || cfg.Org == "" || cfg.Project == "" {
				res := QuerySentryIssuesOutput{
					Success: false,
					Error:   "Sentry 未完整配置 (缺少 auth_token, org 或 project)",
				}
				b, _ := json.Marshal(res)
				return string(b), nil
			}

			baseURL := strings.TrimRight(cfg.BaseUrl, "/")

			traceID := strings.TrimSpace(input.TraceID)
			q := strings.TrimSpace(input.Query)
			if traceID != "" {
				q = fmt.Sprintf("trace_id:%s", traceID)
			} else if len(q) == 32 && !strings.Contains(q, " ") && !strings.Contains(q, ":") {
				// 容错：如果把裸 32 位 trace_id 直接填入 query，自动添加 trace_id: 前缀
				q = fmt.Sprintf("trace_id:%s", q)
			} else if q == "" {
				q = "is:unresolved"
			}
			limit := input.Limit
			if limit <= 0 {
				limit = 5
			} else if limit > 20 {
				limit = 20
			}

			reqURL := fmt.Sprintf("%s/api/0/projects/%s/%s/issues/?query=%s&limit=%d",
				baseURL,
				url.PathEscape(cfg.Org),
				url.PathEscape(cfg.Project),
				url.QueryEscape(q),
				limit,
			)

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
			if err != nil {
				return "", fmt.Errorf("create sentry request failed: %w", err)
			}
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.AuthToken))

			resp, err := getSentryClient().Do(req)
			if err != nil {
				res := QuerySentryIssuesOutput{
					Success: false,
					Error:   fmt.Sprintf("请求 Sentry API 失败: %v", err),
				}
				b, _ := json.Marshal(res)
				return string(b), nil
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return "", fmt.Errorf("read sentry response failed: %w", err)
			}

			if resp.StatusCode != http.StatusOK {
				res := QuerySentryIssuesOutput{
					Success: false,
					Error:   fmt.Sprintf("Sentry API 返回错误状态码 %d: %s", resp.StatusCode, string(body)),
				}
				b, _ := json.Marshal(res)
				return string(b), nil
			}

			var rawIssues []map[string]interface{}
			if err := json.Unmarshal(body, &rawIssues); err != nil {
				return "", fmt.Errorf("parse sentry issues json failed: %w", err)
			}

			issueItems := make([]*SentryIssueItem, 0, len(rawIssues))
			for _, it := range rawIssues {
				item := &SentryIssueItem{
					ID:        fmt.Sprintf("%v", it["id"]),
					ShortID:   fmt.Sprintf("%v", it["shortId"]),
					Title:     fmt.Sprintf("%v", it["title"]),
					Culprit:   fmt.Sprintf("%v", it["culprit"]),
					Level:     fmt.Sprintf("%v", it["level"]),
					Status:    fmt.Sprintf("%v", it["status"]),
					Count:     fmt.Sprintf("%v", it["count"]),
					FirstSeen: fmt.Sprintf("%v", it["firstSeen"]),
					LastSeen:  fmt.Sprintf("%v", it["lastSeen"]),
					Permalink: fmt.Sprintf("%v", it["permalink"]),
				}
				if uc, ok := it["userCount"].(float64); ok {
					item.UserCount = int(uc)
				}
				issueItems = append(issueItems, item)
			}

			out := QuerySentryIssuesOutput{
				Success: true,
				Total:   len(issueItems),
				Issues:  issueItems,
				Message: fmt.Sprintf("成功从 Sentry 查询到 %d 条未解决异常 Issue", len(issueItems)),
			}

			outBytes, _ := json.MarshalIndent(out, "", "  ")
			return string(outBytes), nil
		},
	)

	if err != nil {
		g.Log().Fatalf(context.Background(), "create query_sentry_issues tool failed: %v", err)
	}
	return t
}

// NewGetSentryIssueDetailTool 创建获取 Sentry Issue 堆栈详情的 Eino 工具
func NewGetSentryIssueDetailTool() tool.InvokableTool {
	t, err := utils.InferOptionableTool(
		"get_sentry_issue_detail",
		"根据 Sentry Issue ID 深入获取该报错事件的详细异常类型、错误描述与完整代码调用栈(Stacktrace)。用于定位故障发生的具体代码文件与行号。",
		func(ctx context.Context, input *GetSentryIssueDetailInput, opts ...tool.Option) (string, error) {
			cfg := config.C.Sentry
			if cfg.AuthToken == "" {
				res := SentryIssueDetailOutput{
					Success: false,
					Error:   "Sentry 未配置 auth_token",
				}
				b, _ := json.Marshal(res)
				return string(b), nil
			}

			baseURL := strings.TrimRight(cfg.BaseUrl, "/")
			if baseURL == "" {
				baseURL = "https://sentry.io"
			}

			issueID := strings.TrimSpace(input.IssueID)
			if issueID == "" {
				return "", fmt.Errorf("issue_id 不能为空")
			}

			reqURL := fmt.Sprintf("%s/api/0/issues/%s/events/latest/", baseURL, url.PathEscape(issueID))
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
			if err != nil {
				return "", fmt.Errorf("create sentry detail request failed: %w", err)
			}
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.AuthToken))

			resp, err := getSentryClient().Do(req)
			if err != nil {
				res := SentryIssueDetailOutput{
					Success: false,
					Error:   fmt.Sprintf("请求 Sentry Event API 失败: %v", err),
				}
				b, _ := json.Marshal(res)
				return string(b), nil
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return "", fmt.Errorf("read sentry detail response failed: %w", err)
			}

			if resp.StatusCode != http.StatusOK {
				res := SentryIssueDetailOutput{
					Success: false,
					Error:   fmt.Sprintf("Sentry API 返回错误状态码 %d: %s", resp.StatusCode, string(body)),
				}
				b, _ := json.Marshal(res)
				return string(b), nil
			}

			var eventMap map[string]interface{}
			if err := json.Unmarshal(body, &eventMap); err != nil {
				return "", fmt.Errorf("parse sentry event json failed: %w", err)
			}

			eventIDStr := ""
			if eid, ok := eventMap["eventID"].(string); ok {
				eventIDStr = eid
			} else if idStr, ok := eventMap["id"].(string); ok {
				eventIDStr = idStr
			}

			traceIDStr := ""
			if contexts, ok := eventMap["contexts"].(map[string]interface{}); ok {
				if trace, ok := contexts["trace"].(map[string]interface{}); ok {
					if tid, ok := trace["trace_id"].(string); ok {
						traceIDStr = tid
					}
				}
			}

			out := SentryIssueDetailOutput{
				Success:   true,
				IssueID:   issueID,
				EventID:   eventIDStr,
				TraceID:   traceIDStr,
				Title:     fmt.Sprintf("%v", eventMap["title"]),
				Message:   fmt.Sprintf("%v", eventMap["message"]),
				Permalink: fmt.Sprintf("%s/issues/%s/", strings.Replace(baseURL, "https://sentry.io", fmt.Sprintf("https://%s.sentry.io", cfg.Org), 1), issueID),
			}

			// 解析 entries 中的 exception 与 stacktrace
			if entries, ok := eventMap["entries"].([]interface{}); ok {
				for _, rawEntry := range entries {
					entry, ok := rawEntry.(map[string]interface{})
					if !ok {
						continue
					}
					entryType, _ := entry["type"].(string)
					if entryType == "exception" {
						data, _ := entry["data"].(map[string]interface{})
						if values, ok := data["values"].([]interface{}); ok && len(values) > 0 {
							valMap, _ := values[0].(map[string]interface{})
							exType, _ := valMap["type"].(string)
							exVal, _ := valMap["value"].(string)
							out.Exception = fmt.Sprintf("%s: %s", exType, exVal)

							if st, ok := valMap["stacktrace"].(map[string]interface{}); ok {
								if frames, ok := st["frames"].([]interface{}); ok {
									frameStrs := make([]string, 0, len(frames))
									for _, f := range frames {
										fMap, ok := f.(map[string]interface{})
										if !ok {
											continue
										}
										filename, _ := fMap["filename"].(string)
										function, _ := fMap["function"].(string)
										lineNo := fMap["lineNo"]
										colNo := fMap["colNo"]
										frameStrs = append(frameStrs, fmt.Sprintf("%s:%v:%v in %s()", filename, lineNo, colNo, function))
									}
									out.StackTrace = frameStrs
								}
							}
						}
					}
				}
			}

			outBytes, _ := json.MarshalIndent(out, "", "  ")
			return string(outBytes), nil
		},
	)

	if err != nil {
		g.Log().Fatalf(context.Background(), "create get_sentry_issue_detail tool failed: %v", err)
	}
	return t
}
