package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"mailnest-be/internal/api"
	"mailnest-be/internal/config"
)

func main() {
	configPath := os.Getenv("MAILNEST_CONFIG")
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("加载配置失败：%v", err)
	}

	app, err := api.NewApp(cfg)
	if err != nil {
		log.Fatalf("初始化应用失败：%v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app.StartBackgroundTasks(ctx)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Mail Nest 后端服务已启动 addr=%s", addr)
	if err := http.ListenAndServe(addr, app.Routes()); err != nil {
		log.Fatalf("后端服务异常退出：%v", err)
	}
}
