package main

import (
	"context"
	"time"

	"SuperBizAgent/internal/config"
	"SuperBizAgent/internal/controller/chat"
	"SuperBizAgent/internal/logic/kafka"
	"SuperBizAgent/pkg/client"
	"SuperBizAgent/pkg/middleware"

	"github.com/getsentry/sentry-go"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"
)

func main() {
	ctx := gctx.New()
	config.MustLoad(ctx)

	// 初始化 Sentry 异常监控上报 SDK
	if config.C.Sentry.Dsn != "" {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              config.C.Sentry.Dsn,
			Environment:      "production",
			AttachStacktrace: true,
			TracesSampleRate: config.C.Sentry.TracesSampleRate,
		}); err != nil {
			g.Log().Warningf(ctx, "Sentry SDK 初始化失败: %v", err)
		} else {
			defer sentry.Flush(2 * time.Second)
			g.Log().Infof(ctx, "Sentry 异常与链路监控初始化成功 (TracesSampleRate: %v)", config.C.Sentry.TracesSampleRate)
		}
	}

	// 初始化全局 GORM 数据库连接池
	if _, err := client.InitMySQL(config.C.MySQL.DSN); err != nil {
		g.Log().Warningf(ctx, "初始化 GORM 数据库失败: %v", err)
	}

	// 启动后台 Kafka 异步文档向量化消费服务
	consumerCtx, cancelConsumer := context.WithCancel(ctx)
	defer cancelConsumer()
	consumer, err := kafka.InitAndStartConsumer(consumerCtx)
	if err != nil {
		g.Log().Warningf(ctx, "启动后台 Kafka 消费者失败: %v", err)
	} else if consumer != nil {
		defer consumer.Close()
	}

	s := g.Server()
	s.Group("/api", func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.CORSMiddleware)
		group.Middleware(middleware.ResponseMiddleware)
		group.Bind(chat.NewV1())
	})
	s.SetPort(6872)
	s.Run()
}
