package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mailnest-be/internal/api"
	"mailnest-be/internal/buildinfo"
	"mailnest-be/internal/config"
	"mailnest-be/internal/storage"
)

func main() {
	versionFlag := flag.Bool("version", false, "显示后端版本信息后退出")
	prepareSearchIndex := flag.Bool("prepare-search-index", false, "准备 MySQL 邮件全文索引后退出（执行前须备份数据库）")
	flag.Parse()
	info := buildinfo.Current()
	if *versionFlag {
		fmt.Printf("Mail Nest 后端 version=%s commit=%s buildTime=%s\n", info.Version, info.Commit, info.BuildTime)
		return
	}
	log.Printf("Mail Nest 后端版本：version=%s commit=%s buildTime=%s", info.Version, info.Commit, info.BuildTime)
	configPath := os.Getenv("MAILNEST_CONFIG")
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("加载配置失败：%v", err)
	}

	if *prepareSearchIndex {
		store, err := storage.OpenWithOptions(storage.DatabaseOptions{Driver: cfg.Database.Driver, DSN: cfg.Database.DSN, Path: cfg.Database.Path})
		if err != nil {
			log.Fatalf("打开数据库失败：%v", err)
		}
		defer store.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := store.PrepareMailSearchIndex(ctx); err != nil {
			log.Fatalf("准备邮件全文索引失败：%v", err)
		}
		log.Print("邮件全文索引准备完成，请重启服务以检测并启用索引")
		return
	}
	app, err := api.NewApp(cfg)
	if err != nil {
		log.Fatalf("初始化应用失败：%v", err)
	}
	defer app.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app.StartBackgroundTasks(ctx)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Mail Nest 后端服务已启动 addr=%s", addr)
	server := &http.Server{Addr: addr, Handler: app.Routes(), ReadHeaderTimeout: 10 * time.Second}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("等待请求退出超时，关闭连接 err=%v", err)
			_ = server.Close()
		}
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("后端服务异常退出：%v", err)
		stop()
	}
	<-shutdownDone
}
