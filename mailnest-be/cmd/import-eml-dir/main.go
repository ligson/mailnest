package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mailnest-be/internal/config"
	"mailnest-be/internal/mail"
	"mailnest-be/internal/oauth"
	"mailnest-be/internal/storage"
)

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	dir := flag.String("dir", "", "EML 文件目录")
	folder := flag.String("folder", "INBOX", "导入目录")
	userID := flag.Int64("user", 0, "用户 ID")
	accountID := flag.Int64("account", 0, "邮箱账号 ID")
	syncUploadSessions := flag.Bool("sync-upload-sessions", true, "导入成功后同步浏览器断点上传会话状态")
	flag.Parse()

	if *dir == "" || *userID <= 0 || *accountID <= 0 {
		log.Fatal("参数错误：必须提供 -dir、-user 和 -account")
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
	var total, inserted, duplicated, failed int
	err = filepath.WalkDir(*dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := entry.Name()
		if entry.IsDir() {
			if name == "__MACOSX" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, "._") || !strings.EqualFold(filepath.Ext(name), ".eml") {
			return nil
		}
		total++
		raw, err := os.ReadFile(path)
		if err != nil {
			failed++
			log.Printf("读取 EML 失败 path=%s err=%v", path, err)
			return nil
		}
		size := int64(len(raw))
		result, err := service.ImportEML(*userID, *accountID, *folder, raw)
		raw = nil
		if err != nil {
			failed++
			log.Printf("导入 EML 失败 path=%s err=%v", path, err)
			return nil
		}
		if result.Inserted {
			inserted++
		} else {
			duplicated++
		}
		if *syncUploadSessions {
			if err := markMatchingUploadSessionsImported(cfg.App.DataDir, *userID, *accountID, *folder, entry.Name(), size, result.Message.ID, result.Inserted); err != nil {
				log.Printf("同步 EML 上传会话状态失败 path=%s err=%v", path, err)
			}
		}
		log.Printf("导入 EML 完成 path=%s messageID=%d inserted=%t", path, result.Message.ID, result.Inserted)
		return nil
	})
	if err != nil {
		log.Fatalf("遍历 EML 目录失败：%v", err)
	}
	fmt.Printf("导入完成 total=%d inserted=%d duplicated=%d failed=%d\n", total, inserted, duplicated, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

type uploadSessionMeta struct {
	UploadID     string `json:"uploadId"`
	UserID       int64  `json:"userId"`
	AccountID    int64  `json:"accountId"`
	Folder       string `json:"folder"`
	Filename     string `json:"filename"`
	Size         int64  `json:"size"`
	LastModified int64  `json:"lastModified"`
	FileKey      string `json:"fileKey"`
	Status       string `json:"status"`
	Error        string `json:"error"`
	Inserted     bool   `json:"inserted"`
	MessageID    int64  `json:"messageId"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

func markMatchingUploadSessionsImported(dataDir string, userID, accountID int64, folder, filename string, size int64, messageID int64, inserted bool) error {
	importRoot := filepath.Join(dataDir, "imports", "users", fmt.Sprint(userID))
	entries, err := os.ReadDir(importRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(importRoot, entry.Name(), "meta.json")
		content, err := os.ReadFile(metaPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		var meta uploadSessionMeta
		if err := json.Unmarshal(content, &meta); err != nil {
			return fmt.Errorf("解析上传会话 %s 失败：%w", metaPath, err)
		}
		if meta.UserID != userID ||
			meta.AccountID != accountID ||
			meta.Folder != folder ||
			!sameImportFilename(meta.Filename, filename) ||
			meta.Size != size ||
			meta.Status == "imported" {
			continue
		}
		meta.Status = "imported"
		meta.Error = ""
		meta.Inserted = inserted
		meta.MessageID = messageID
		meta.UpdatedAt = time.Now().Format(time.RFC3339)
		nextContent, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(metaPath, nextContent, 0o600); err != nil {
			return err
		}
		uploadPath := filepath.Join(importRoot, entry.Name(), "upload.eml")
		if err := os.Remove(uploadPath); err != nil && !os.IsNotExist(err) {
			log.Printf("清理已导入上传临时文件失败 path=%s err=%v", uploadPath, err)
		}
		log.Printf("已同步 EML 上传会话状态 uploadID=%s filename=%s messageID=%d inserted=%t", meta.UploadID, meta.Filename, messageID, inserted)
	}
	return nil
}

func sameImportFilename(sessionFilename, localFilename string) bool {
	sessionFilename = strings.TrimSpace(sessionFilename)
	localFilename = strings.TrimSpace(localFilename)
	if sessionFilename == localFilename {
		return true
	}
	if len(localFilename) > 3 &&
		localFilename[0] >= '0' && localFilename[0] <= '9' &&
		localFilename[1] >= '0' && localFilename[1] <= '9' &&
		localFilename[2] == '-' {
		return sessionFilename == strings.TrimSpace(localFilename[3:])
	}
	return false
}
