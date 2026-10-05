package middleware

import (
	"fmt"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/net/gtrace"
)

// CORSMiddleware 处理CORS跨域请求
func CORSMiddleware(r *ghttp.Request) {
	r.Response.CORSDefault()
	r.Middleware.Next()
}

type Response struct {
	TraceID string      `json:"trace_id,omitempty" dc:"链路追踪ID"`
	Message string      `json:"message"            dc:"消息提示"`
	Data    interface{} `json:"data"               dc:"执行结果"`
}

func ResponseMiddleware(r *ghttp.Request) {
	// 为当前 HTTP 请求启动 Sentry APM 事务 (Root Span)
	hub := sentry.CurrentHub().Clone()
	ctx := sentry.SetHubOnContext(r.Context(), hub)
	span := sentry.StartSpan(ctx, "http.server",
		sentry.TransactionName(fmt.Sprintf("%s %s", r.Method, r.URL.Path)),
		sentry.ContinueFromRequest(r.Request),
	)
	defer span.Finish()

	r.SetCtx(span.Context())
	r.Middleware.Next()

	traceID := span.TraceID.String()
	if traceID == "" || traceID == "00000000000000000000000000000000" {
		traceID = gtrace.GetTraceID(r.Context())
	}
	if traceID != "" {
		r.Response.Header().Set("X-Trace-Id", traceID)
	}

	var (
		msg string
		res = r.GetHandlerResponse()
		err = r.GetError()
	)
	if err != nil {
		span.Status = sentry.SpanStatusInternalError
		msg = err.Error()
		// 发生错误时自动上报 Sentry，关联 trace_id 与调用上下文
		hub.WithScope(func(scope *sentry.Scope) {
			if traceID != "" {
				scope.SetTag("trace_id", traceID)
			}
			scope.SetRequest(r.Request)
			hub.CaptureException(err)
		})
	} else {
		span.Status = sentry.SpanStatusOK
		msg = "OK"
	}
	if strings.Contains(r.Response.Header().Get("Content-Type"), "text/event-stream") {
		return
	}
	r.Response.WriteJson(Response{
		TraceID: traceID,
		Message: msg,
		Data:    res,
	})
}
