package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"mailnest-be/internal/config"
	"mailnest-be/internal/mail"
	"mailnest-be/internal/oauth"
	"mailnest-be/internal/storage"
)

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	userID := flag.Int64("user", 0, "用户 ID")
	accountID := flag.Int64("account", 0, "邮箱账号 ID")
	importedOnly := flag.Bool("imported-only", true, "仅修复 EML 导入邮件")
	batchSize := flag.Int("batch-size", 500, "每批读取数量，最大 1000")
	flag.Parse()

	if *userID <= 0 || *accountID <= 0 {
		log.Fatal("参数错误：必须提供 -user 和 -account")
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败：%v", err)
	}
	store, err := storage.OpenWithOptions(storage.DatabaseOptions{
		Driver:       cfg.Database.Driver,
		DSN:          cfg.Database.DSN,
		Path:         cfg.Database.Path,
		MaxOpenConns: cfg.Database.MaxOpenConns,
		MaxIdleConns: cfg.Database.MaxIdleConns,
	})
	if err != nil {
		log.Fatalf("打开数据库失败：%v", err)
	}
	defer store.Close()

	service := mail.NewService(store, nil, oauth.NewMicrosoftExchanger(cfg.OAuth.Microsoft), cfg.App.DataDir, cfg.App.CredentialSecret)
	result, err := service.RepairMissingSentAt(*userID, *accountID, *importedOnly, *batchSize)
	if err != nil {
		log.Fatalf("修复邮件发送时间失败：%v", err)
	}
	fmt.Printf("修复完成 checked=%d repaired=%d noValidDate=%d readErrors=%d databaseSkips=%d\n",
		result.Checked,
		result.Repaired,
		result.NoValidDate,
		result.ReadErrors,
		result.DatabaseSkips,
	)
	if result.ReadErrors > 0 {
		os.Exit(1)
	}
}
