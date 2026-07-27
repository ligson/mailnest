package api

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

func (a *App) handleImportEMLMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxImportEMLBatchBytes+maxImportEMLMultipartOverhead)
	files, err := readImportEMLFiles(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	accountID, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("accountId")), 10, 64)
	if err != nil || accountID <= 0 {
		response.Error(w, http.StatusBadRequest, "请选择导入邮箱账号")
		return
	}
	folder := strings.TrimSpace(r.FormValue("folder"))
	if folder == "" {
		folder = "INBOX"
	}
	payload := importEMLBatchPayload(userID, accountID, folder, files, a)
	if payload.accountNotFound {
		response.Error(w, http.StatusNotFound, "邮箱账号不存在")
		return
	}
	log.Printf("EML 邮件批量导入完成 userID=%d accountID=%d total=%d inserted=%d duplicated=%d failed=%d", userID, accountID, payload.total, payload.inserted, payload.duplicated, payload.failed)
	response.OK(w, "导入完成", payload.data)
}

type importEMLBatchResult struct {
	data            map[string]any
	total           int
	inserted        int
	duplicated      int
	failed          int
	accountNotFound bool
}

func importEMLBatchPayload(userID, accountID int64, folder string, files []importEMLUpload, a *App) importEMLBatchResult {
	items := make([]map[string]any, 0, len(files))
	var firstMessage any
	var firstInserted any
	result := importEMLBatchResult{total: len(files)}
	for _, file := range files {
		item := map[string]any{
			"filename": file.Filename,
			"success":  false,
			"inserted": false,
			"message":  nil,
			"error":    nil,
		}
		if file.Error != "" {
			item["error"] = file.Error
			result.failed++
			items = append(items, item)
			continue
		}
		data, err := readImportEMLPart(file.Header)
		if err != nil {
			item["error"] = err.Error()
			result.failed++
			items = append(items, item)
			continue
		}
		imported, err := a.mailService.ImportEML(userID, accountID, folder, data)
		data = nil
		if errors.Is(err, storage.ErrNotFound) {
			result.accountNotFound = true
			return result
		}
		if err != nil {
			item["error"] = err.Error()
			result.failed++
			items = append(items, item)
			continue
		}
		messagePayload := messageListPayload(imported.Message)
		item["success"] = true
		item["inserted"] = imported.Inserted
		item["message"] = messagePayload
		if imported.Inserted {
			result.inserted++
		} else {
			result.duplicated++
		}
		if firstMessage == nil {
			firstMessage = messagePayload
			firstInserted = imported.Inserted
		}
		items = append(items, item)
	}
	result.data = map[string]any{
		"total":          result.total,
		"successCount":   result.inserted + result.duplicated,
		"insertedCount":  result.inserted,
		"duplicateCount": result.duplicated,
		"failedCount":    result.failed,
		"items":          items,
		"inserted":       firstInserted,
		"message":        firstMessage,
		"batch":          result.total > 1,
		"maxFileSizeMB":  maxImportEMLBytes >> 20,
		"maxBatchCount":  nil,
		"maxBatchSizeMB": maxImportEMLBatchBytes >> 20,
	}
	return result
}
