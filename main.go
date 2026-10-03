package main

import (
	"context"

	"SuperBizAgent/internal/config"
	"SuperBizAgent/internal/controller/chat"
	"SuperBizAgent/internal/logic/kafka"
	"SuperBizAgent/pkg/middleware"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"
)

func main() {
	ctx := gctx.New()
	fileDir, err := g.Cfg().Get(ctx, "file_dir")
	if err != nil {
		panic(err)
	}
	config.FileDir = fileDir.String()

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

