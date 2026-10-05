package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"SuperBizAgent/internal/config"

	"github.com/getsentry/sentry-go"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
)

func main() {
	ctx := context.Background()

	// 确保加载配置文件
	if adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile); ok {
		_ = adapter.AddPath("etc/config", "etc")
	}
	config.MustLoad(ctx)

	dsn := config.C.Sentry.Dsn
	if dsn == "" {
		panic("Sentry DSN 为空，请检查 etc/config/config.yaml")
	}

	fmt.Println("==================================================================")
	fmt.Printf("🚀 正在向 Sentry (%s) 发送模拟排障错误事件...\n", config.C.Sentry.Org)
	fmt.Println("==================================================================")

	sampleRate := config.C.Sentry.TracesSampleRate
	if sampleRate == 0 {
		sampleRate = 1.0
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      "production",
		AttachStacktrace: true,
		TracesSampleRate: sampleRate,
	})
	if err != nil {
		panic(fmt.Sprintf("Sentry SDK 初始化失败: %v", err))
	}
	defer sentry.Flush(5 * time.Second)

	fmt.Printf("📊 Sentry APM 链路采样率 (TracesSampleRate): %.2f (已自 config.yaml 成功加载)\n\n", sampleRate)

	// =========================================================================
	// 场景 4 (重点): 完整的微服务端到端链路追踪 (Transaction + Child Spans + 异常报错)
	// =========================================================================
	hub := sentry.CurrentHub().Clone()
	rootCtx := sentry.SetHubOnContext(ctx, hub)

	// 1. 开启根事务 (Root Transaction)
	txSpan := sentry.StartSpan(rootCtx, "http.server",
		sentry.TransactionName("POST /api/order/checkout"),
	)
	traceID := txSpan.TraceID.String()

	hub.Scope().SetTag("trace_id", traceID)
	hub.Scope().SetTag("service", "order-center")
	hub.Scope().SetTag("environment", "production")
	hub.Scope().SetExtra("order_no", "ORD-20261005-8888")
	hub.Scope().SetExtra("user_id", "usr_vip_8888")

	fmt.Println("🚀 模拟订单创建全链路 (POST /api/order/checkout)...")
	fmt.Printf("🔗 本次链路全局 TraceID: %s\n", traceID)

	// 2. 子步骤 1: 鉴权校验 Span
	spanAuth := txSpan.StartChild("auth.verify_token")
	spanAuth.Description = "校验用户 JWT 令牌与 VIP 权限"
	time.Sleep(35 * time.Millisecond) // 模拟耗时 35ms
	spanAuth.Status = sentry.SpanStatusOK
	spanAuth.Finish()
	fmt.Println("  ├─ [Span 1] auth.verify_token (35ms) -> OK")

	// 3. 子步骤 2: Redis 锁定库存 Span
	spanStock := txSpan.StartChild("cache.redis")
	spanStock.Description = "Redis 分布式锁扣减商品库存 sku_7721"
	spanStock.SetTag("db.system", "redis")
	time.Sleep(50 * time.Millisecond) // 模拟耗时 50ms
	spanStock.Status = sentry.SpanStatusOK
	spanStock.Finish()
	fmt.Println("  ├─ [Span 2] cache.redis lock_stock (50ms) -> OK")

	// 4. 子步骤 3: 数据库插入订单（此处发生严重死锁与唯一索引冲突错误！）
	spanDB := txSpan.StartChild("db.mysql")
	spanDB.Description = "MySQL 执行 INSERT INTO orders 事务提交"
	spanDB.SetTag("db.system", "mysql")
	time.Sleep(75 * time.Millisecond) // 模拟耗时 75ms

	orderErr := errors.New("OrderDuplicateException: Duplicate entry 'ORD-20261005-8888' for key 'orders.uk_order_no' at order_dao.go:94")
	spanDB.Status = sentry.SpanStatusInternalError
	spanDB.Finish()
	fmt.Println("  └─ [Span 3] db.mysql insert (75ms) -> ❌ 发生订单编号冲突异常！")

	// 5. 捕获异常，异常会自动与当前事务及 trace_id 绑定
	txSpan.Status = sentry.SpanStatusInternalError
	eventID := hub.CaptureException(orderErr)
	txSpan.Finish() // 结束根事务，发送整条瀑布流到 Sentry

	fmt.Printf("\n✅ [成功上报全链路瀑布图] EventID: %v\n", *eventID)
	fmt.Printf("👉 Sentry 链路瀑布流查看地址 (Trace View):\n   https://%s.sentry.io/performance/trace/%s/\n", config.C.Sentry.Org, traceID)
	fmt.Printf("👉 Sentry Issue 详情查看地址:\n   https://%s.sentry.io/issues/?query=trace_id:%s\n", config.C.Sentry.Org, traceID)

	fmt.Println("\n⏳ 正在执行 Sentry 缓冲队列刷盘 (Flush)...")
	sentry.Flush(4 * time.Second)
	fmt.Println("🎉 全链路瀑布图与异常已成功送达 Sentry！")
}

func simulateNilPointerPanic() {
	var configMap map[string]*string
	// 触发空指针异常
	_ = *configMap["missing_key"]
}
