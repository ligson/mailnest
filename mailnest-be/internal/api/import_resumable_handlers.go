package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

const importEMLChunkBytes int64 = 16 << 20

const importEMLChunkOverhead int64 = 1 << 20

type importEMLUploadCreateRequest struct {
	AccountID    string `json:"accountId"`
	Folder       string `json:"folder"`
	Filename     string `json:"filename"`
	Size         int64  `json:"size"`
	LastModified int64  `json:"lastModified"`
	FileKey      string `json:"fileKey"`
}

type importEMLUploadSession struct {
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

func (a *App) handleCreateImportEMLUpload(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	var req importEMLUploadCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	accountID, err := strconv.ParseInt(strings.TrimSpace(req.AccountID), 10, 64)
	if err != nil || accountID <= 0 {
		response.Error(w, http.StatusBadRequest, "请选择导入邮箱账号")
		return
	}
	if _, err := a.store.FindMailAccountByID(userID, accountID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "邮箱账号不存在")
			return
		}
		response.Error(w, http.StatusInternalServerError, "读取邮箱账号失败")
		return
	}
	filename := filepath.Base(strings.TrimSpace(req.Filename))
	if filename == "" {
		response.Error(w, http.StatusBadRequest, "EML 文件名为空")
		return
	}
	if !strings.EqualFold(filepath.Ext(filename), ".eml") {
		response.Error(w, http.StatusBadRequest, "只支持导入 .eml 文件")
		return
	}
	if req.Size <= 0 {
		response.Error(w, http.StatusBadRequest, "EML 文件为空")
		return
	}
	if req.Size > maxImportEMLBytes {
		response.Error(w, http.StatusBadRequest, fmt.Sprintf("EML 文件不能超过 %d GB", maxImportEMLBytes>>30))
		return
	}
	folder := strings.TrimSpace(req.Folder)
	if folder == "" {
		folder = "INBOX"
	}
	session := importEMLUploadSession{
		UploadID:     importEMLUploadID(userID, accountID, folder, filename, req.Size, req.LastModified, req.FileKey),
		UserID:       userID,
		AccountID:    accountID,
		Folder:       folder,
		Filename:     filename,
		Size:         req.Size,
		LastModified: req.LastModified,
		FileKey:      strings.TrimSpace(req.FileKey),
		Status:       "uploading",
	}
	existing, err := a.readImportEMLUploadSession(userID, session.UploadID)
	if err == nil {
		session = existing
	} else if !errors.Is(err, os.ErrNotExist) {
		response.Error(w, http.StatusInternalServerError, "读取上传会话失败")
		return
	}
	if strings.TrimSpace(session.CreatedAt) == "" {
		session.CreatedAt = time.Now().Format(time.RFC3339)
	}
	session.UpdatedAt = time.Now().Format(time.RFC3339)
	if err := a.writeImportEMLUploadSession(session); err != nil {
		response.Error(w, http.StatusInternalServerError, "保存上传会话失败")
		return
	}
	uploadedBytes, _ := a.importEMLUploadSize(userID, session.UploadID)
	if session.Status == "imported" && uploadedBytes == 0 {
		uploadedBytes = session.Size
	}
	if uploadedBytes > session.Size {
		uploadedBytes = session.Size
		if err := os.Truncate(a.importEMLUploadDataPath(userID, session.UploadID), session.Size); err != nil {
			log.Printf("修正 EML 断点上传文件大小失败 userID=%d uploadID=%s err=%v", userID, session.UploadID, err)
		}
	}
	log.Printf("EML 断点上传会话准备完成 userID=%d accountID=%d uploadID=%s filename=%s uploaded=%d size=%d status=%s", userID, accountID, session.UploadID, session.Filename, uploadedBytes, session.Size, session.Status)
	response.OK(w, "上传会话已准备", importEMLUploadPayload(session, uploadedBytes))
}

func (a *App) handleImportEMLUploadHeartbeat(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	session, ok := a.importEMLUploadSessionFromRoute(w, userID, r)
	if !ok {
		return
	}
	uploadedBytes, _ := a.importEMLUploadSize(userID, session.UploadID)
	if session.Status == "imported" && uploadedBytes == 0 {
		uploadedBytes = session.Size
	}
	response.OK(w, "上传会话正常", importEMLUploadPayload(session, uploadedBytes))
}

func (a *App) handleImportEMLUploadChunk(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	session, ok := a.importEMLUploadSessionFromRoute(w, userID, r)
	if !ok {
		return
	}
	offset, err := strconv.ParseInt(strings.TrimSpace(r.Header.Get("X-Upload-Offset")), 10, 64)
	if err != nil || offset < 0 {
		response.Error(w, http.StatusBadRequest, "上传偏移量格式错误")
		return
	}
	uploadedBytes, _ := a.importEMLUploadSize(userID, session.UploadID)
	if session.Status == "imported" {
		response.OK(w, "文件已导入完成", importEMLUploadPayload(session, session.Size))
		return
	}
	if uploadedBytes > session.Size {
		if err := os.Truncate(a.importEMLUploadDataPath(userID, session.UploadID), session.Size); err != nil {
			response.Error(w, http.StatusInternalServerError, "修正上传文件失败")
			return
		}
		uploadedBytes = session.Size
	}
	if offset != uploadedBytes {
		response.JSON(w, http.StatusConflict, false, "上传偏移量不一致，请按服务端进度续传", importEMLUploadPayload(session, uploadedBytes))
		return
	}
	if uploadedBytes >= session.Size {
		response.OK(w, "文件已上传完成", importEMLUploadPayload(session, uploadedBytes))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, importEMLChunkBytes+importEMLChunkOverhead)
	defer r.Body.Close()
	if err := os.MkdirAll(a.importEMLUploadDir(userID, session.UploadID), 0o755); err != nil {
		response.Error(w, http.StatusInternalServerError, "创建上传目录失败")
		return
	}
	file, err := os.OpenFile(a.importEMLUploadDataPath(userID, session.UploadID), os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "打开上传文件失败")
		return
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		_ = file.Close()
		response.Error(w, http.StatusInternalServerError, "定位上传文件失败")
		return
	}
	written, copyErr := io.Copy(file, r.Body)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		response.Error(w, http.StatusBadRequest, "保存上传分片失败")
		return
	}
	if written <= 0 {
		response.Error(w, http.StatusBadRequest, "上传分片为空")
		return
	}
	uploadedBytes = offset + written
	if uploadedBytes > session.Size {
		_ = os.Truncate(a.importEMLUploadDataPath(userID, session.UploadID), session.Size)
		response.Error(w, http.StatusBadRequest, "上传内容超过文件大小")
		return
	}
	session.Status = "uploading"
	if uploadedBytes == session.Size {
		session.Status = "uploaded"
	}
	session.UpdatedAt = time.Now().Format(time.RFC3339)
	if err := a.writeImportEMLUploadSession(session); err != nil {
		response.Error(w, http.StatusInternalServerError, "更新上传会话失败")
		return
	}
	log.Printf("EML 分片上传成功 userID=%d uploadID=%s uploaded=%d size=%d status=%s", userID, session.UploadID, uploadedBytes, session.Size, session.Status)
	response.OK(w, "分片上传成功", importEMLUploadPayload(session, uploadedBytes))
}

func (a *App) handleFinishImportEMLUpload(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	session, ok := a.importEMLUploadSessionFromRoute(w, userID, r)
	if !ok {
		return
	}
	uploadedBytes, _ := a.importEMLUploadSize(userID, session.UploadID)
	if session.Status == "imported" {
		response.OK(w, "导入完成", map[string]any{
			"filename":      session.Filename,
			"success":       true,
			"inserted":      session.Inserted,
			"message":       nil,
			"uploadedBytes": session.Size,
			"uploadId":      session.UploadID,
		})
		return
	}
	if uploadedBytes != session.Size {
		response.JSON(w, http.StatusConflict, false, "文件还没有上传完成", importEMLUploadPayload(session, uploadedBytes))
		return
	}
	log.Printf("EML 断点上传开始导入 userID=%d accountID=%d uploadID=%s filename=%s size=%d", userID, session.AccountID, session.UploadID, session.Filename, session.Size)
	raw, err := os.ReadFile(a.importEMLUploadDataPath(userID, session.UploadID))
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "读取上传文件失败")
		return
	}
	imported, err := a.mailService.ImportEML(userID, session.AccountID, session.Folder, raw)
	raw = nil
	if err != nil {
		session.Status = "failed"
		session.Error = err.Error()
		session.UpdatedAt = time.Now().Format(time.RFC3339)
		_ = a.writeImportEMLUploadSession(session)
		if errors.Is(err, storage.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "邮箱账号不存在")
			return
		}
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	session.Status = "imported"
	session.Error = ""
	session.Inserted = imported.Inserted
	session.MessageID = imported.Message.ID
	session.UpdatedAt = time.Now().Format(time.RFC3339)
	if err := a.writeImportEMLUploadSession(session); err != nil {
		response.Error(w, http.StatusInternalServerError, "更新上传会话失败")
		return
	}
	if err := os.Remove(a.importEMLUploadDataPath(userID, session.UploadID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("清理 EML 临时上传文件失败 userID=%d uploadID=%s err=%v", userID, session.UploadID, err)
	}
	messagePayload := messageListPayload(imported.Message)
	log.Printf("EML 断点上传导入完成 userID=%d accountID=%d uploadID=%s messageID=%d inserted=%t", userID, session.AccountID, session.UploadID, imported.Message.ID, imported.Inserted)
	response.OK(w, "导入完成", map[string]any{
		"filename":      session.Filename,
		"success":       true,
		"inserted":      imported.Inserted,
		"message":       messagePayload,
		"uploadedBytes": session.Size,
		"uploadId":      session.UploadID,
	})
}

func (a *App) importEMLUploadSessionFromRoute(w http.ResponseWriter, userID int64, r *http.Request) (importEMLUploadSession, bool) {
	uploadID := strings.TrimSpace(r.PathValue("id"))
	if uploadID == "" {
		response.Error(w, http.StatusBadRequest, "上传会话 ID 不能为空")
		return importEMLUploadSession{}, false
	}
	session, err := a.readImportEMLUploadSession(userID, uploadID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			response.Error(w, http.StatusNotFound, "上传会话不存在")
			return importEMLUploadSession{}, false
		}
		response.Error(w, http.StatusInternalServerError, "读取上传会话失败")
		return importEMLUploadSession{}, false
	}
	return session, true
}

func (a *App) readImportEMLUploadSession(userID int64, uploadID string) (importEMLUploadSession, error) {
	content, err := os.ReadFile(a.importEMLUploadMetaPath(userID, uploadID))
	if err != nil {
		return importEMLUploadSession{}, err
	}
	var session importEMLUploadSession
	if err := json.Unmarshal(content, &session); err != nil {
		return importEMLUploadSession{}, err
	}
	if session.UserID != userID || session.UploadID != uploadID {
		return importEMLUploadSession{}, os.ErrPermission
	}
	return session, nil
}

func (a *App) writeImportEMLUploadSession(session importEMLUploadSession) error {
	dir := a.importEMLUploadDir(session.UserID, session.UploadID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "meta.json"), content, 0o600)
}

func (a *App) importEMLUploadSize(userID int64, uploadID string) (int64, error) {
	stat, err := os.Stat(a.importEMLUploadDataPath(userID, uploadID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	return stat.Size(), nil
}

func importEMLUploadPayload(session importEMLUploadSession, uploadedBytes int64) map[string]any {
	return map[string]any{
		"uploadId":      session.UploadID,
		"filename":      session.Filename,
		"size":          session.Size,
		"uploadedBytes": uploadedBytes,
		"chunkSize":     importEMLChunkBytes,
		"status":        session.Status,
		"error":         session.Error,
	}
}

func (a *App) importEMLUploadDir(userID int64, uploadID string) string {
	return filepath.Join(a.cfg.App.DataDir, "imports", "users", fmt.Sprint(userID), safeUploadID(uploadID))
}

func (a *App) importEMLUploadMetaPath(userID int64, uploadID string) string {
	return filepath.Join(a.importEMLUploadDir(userID, uploadID), "meta.json")
}

func (a *App) importEMLUploadDataPath(userID int64, uploadID string) string {
	return filepath.Join(a.importEMLUploadDir(userID, uploadID), "upload.eml")
}

func importEMLUploadID(userID, accountID int64, folder, filename string, size, lastModified int64, fileKey string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d|%d|%s|%s|%d|%d|%s", userID, accountID, folder, filename, size, lastModified, strings.TrimSpace(fileKey))))
	return hex.EncodeToString(sum[:16])
}

func safeUploadID(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			builder.WriteRune(r)
		}
	}
	if builder.Len() == 0 {
		return "upload"
	}
	return builder.String()
}
