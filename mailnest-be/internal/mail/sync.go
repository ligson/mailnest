package mail

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"mailnest-be/internal/storage"
)

type SyncResult struct {
	JobID           int64
	NewMessageCount int
	Warnings        []string
}

// FullSyncStatus 是前端轮询全量同步进度时使用的状态快照。
type FullSyncStatus struct {
	Status         string
	Total          int
	Processed      int
	NewCount       int
	StartedAt      sql.NullTime
	FinishedAt     sql.NullTime
	Error          sql.NullString
	CleanupEnabled bool
	RetentionDays  int
}

const fullSyncBatchSize = 50

var ErrSyncAlreadyRunning = errors.New("邮箱账号正在收取中")

func (s *Service) SyncInbox(userID, accountID int64) (SyncResult, error) {
	if !s.tryRegisterInboxSync(userID, accountID) {
		log.Printf("邮件手动收取被跳过：账号已有同步任务 userID=%d accountID=%d", userID, accountID)
		return SyncResult{}, ErrSyncAlreadyRunning
	}
	defer s.unregisterInboxSync(userID, accountID)

	account, err := s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return SyncResult{}, err
	}
	return s.syncInbox(account, "manual")
}

func (s *Service) syncInbox(account storage.MailAccount, triggerType string) (SyncResult, error) {
	started := time.Now()
	jobID, err := s.store.CreateSyncJob(account.UserID, account.ID, triggerType, "running")
	if err != nil {
		return SyncResult{}, err
	}
	log.Printf("邮件收取开始 userID=%d accountID=%d jobID=%d trigger=%s", account.UserID, account.ID, jobID, triggerType)
	_ = s.store.CreateSyncJobEvent(jobID, "info", "start", "开始收取邮件", mustJSON(map[string]any{
		"triggerType": triggerType,
		"accountId":   account.ID,
	}))

	config, err := s.accountConfig(account)
	if err != nil {
		log.Printf("邮件收取失败：账号配置不可用 userID=%d accountID=%d jobID=%d err=%v", account.UserID, account.ID, jobID, err)
		_ = s.store.FinishSyncJob(jobID, "failed", 0, err.Error())
		_ = s.store.CreateSyncJobEvent(jobID, "error", "config", err.Error(), mustJSON(nil))
		_ = s.store.UpdateMailAccountSyncStatus(account.UserID, account.ID, "failed", err.Error())
		return SyncResult{JobID: jobID}, err
	}

	newCount := 0
	warnings := make([]string, 0)
	for _, folder := range accountSyncFolders(account) {
		_ = s.store.CreateSyncJobEvent(jobID, "info", "folder", "正在同步文件夹 "+folder, mustJSON(map[string]any{
			"folder": folder,
		}))
		folderConfig := configForFolder(config, folder)
		messages, err := s.fetcher.FetchFolder(folderConfig)
		if err != nil {
			if shouldSkipMissingOptionalFolder(folder, err) {
				log.Printf("邮件收取跳过可选文件夹 userID=%d accountID=%d jobID=%d folder=%s", account.UserID, account.ID, jobID, folder)
				warnings = append(warnings, fmt.Sprintf("文件夹 %s 不存在，已跳过", folder))
				_ = s.store.CreateSyncJobEvent(jobID, "warn", "folder", "文件夹 "+folder+" 不存在，已跳过", mustJSON(map[string]any{
					"folder": folder,
				}))
				continue
			}
			log.Printf("邮件收取失败：读取文件夹失败 userID=%d accountID=%d jobID=%d folder=%s err=%v", account.UserID, account.ID, jobID, folder, err)
			_ = s.store.FinishSyncJob(jobID, "failed", newCount, err.Error())
			_ = s.store.CreateSyncJobEvent(jobID, "error", "folder", err.Error(), mustJSON(map[string]any{
				"folder": folder,
			}))
			_ = s.store.UpdateMailAccountSyncStatus(account.UserID, account.ID, "failed", err.Error())
			return SyncResult{JobID: jobID, NewMessageCount: newCount, Warnings: warnings}, err
		}
		for _, fetched := range messages {
			inserted, err := s.saveMessage(account.UserID, account.ID, folder, fetched)
			if err != nil {
				log.Printf("邮件收取失败：保存邮件失败 userID=%d accountID=%d jobID=%d folder=%s uid=%s err=%v", account.UserID, account.ID, jobID, folder, fetched.UID, err)
				_ = s.store.FinishSyncJob(jobID, "failed", newCount, err.Error())
				_ = s.store.CreateSyncJobEvent(jobID, "error", "message", err.Error(), mustJSON(map[string]any{
					"folder": folder,
					"uid":    fetched.UID,
				}))
				_ = s.store.UpdateMailAccountSyncStatus(account.UserID, account.ID, "failed", err.Error())
				return SyncResult{JobID: jobID, NewMessageCount: newCount, Warnings: warnings}, err
			}
			if inserted {
				newCount++
			}
		}
		log.Printf("邮件收取文件夹完成 userID=%d accountID=%d jobID=%d folder=%s fetched=%d newTotal=%d", account.UserID, account.ID, jobID, folder, len(messages), newCount)
		_ = s.store.CreateSyncJobEvent(jobID, "info", "folder", "文件夹 "+folder+" 同步完成", mustJSON(map[string]any{
			"folder": folder,
		}))
	}

	_ = s.store.FinishSyncJob(jobID, "success", newCount, "")
	_ = s.store.CreateSyncJobEvent(jobID, "info", "finish", "收取完成", mustJSON(map[string]any{
		"newMessageCount": newCount,
	}))
	_ = s.store.UpdateMailAccountSyncStatus(account.UserID, account.ID, "success", "")
	log.Printf("邮件收取完成 userID=%d accountID=%d jobID=%d new=%d warnings=%d duration=%s", account.UserID, account.ID, jobID, newCount, len(warnings), time.Since(started))
	return SyncResult{JobID: jobID, NewMessageCount: newCount, Warnings: warnings}, nil
}

func (s *Service) tryRegisterInboxSync(userID, accountID int64) bool {
	s.inboxSyncMu.Lock()
	defer s.inboxSyncMu.Unlock()
	key := fullSyncKey(userID, accountID)
	if _, ok := s.inboxSyncs[key]; ok {
		return false
	}
	s.inboxSyncs[key] = struct{}{}
	return true
}

func (s *Service) unregisterInboxSync(userID, accountID int64) {
	s.inboxSyncMu.Lock()
	defer s.inboxSyncMu.Unlock()
	delete(s.inboxSyncs, fullSyncKey(userID, accountID))
}

func (s *Service) StartFullSync(userID, accountID int64) (FullSyncStatus, error) {
	account, err := s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return FullSyncStatus{}, err
	}
	if account.FullSyncStatus == "running" {
		log.Printf("全量同步请求复用运行中状态 userID=%d accountID=%d processed=%d total=%d", userID, accountID, account.FullSyncProcessed, account.FullSyncTotal)
		return fullSyncStatusFromAccount(account), nil
	}
	if err := s.store.StartMailAccountFullSync(userID, accountID, 0); err != nil {
		return FullSyncStatus{}, err
	}
	cancel := s.registerFullSyncCancel(userID, accountID)

	log.Printf("全量同步已启动 userID=%d accountID=%d", userID, accountID)
	go s.runFullSync(userID, accountID, cancel)

	account, err = s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return FullSyncStatus{}, err
	}
	return fullSyncStatusFromAccount(account), nil
}

func (s *Service) StopFullSync(userID, accountID int64) (FullSyncStatus, error) {
	account, err := s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return FullSyncStatus{}, err
	}
	if account.FullSyncStatus != "running" {
		return fullSyncStatusFromAccount(account), nil
	}
	log.Printf("全量同步收到停止请求 userID=%d accountID=%d", userID, accountID)
	s.cancelFullSync(userID, accountID)
	if err := s.store.FinishMailAccountFullSync(userID, accountID, "cancelled", "用户停止了全量同步"); err != nil {
		return FullSyncStatus{}, err
	}
	account, err = s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return FullSyncStatus{}, err
	}
	return fullSyncStatusFromAccount(account), nil
}

func (s *Service) GetFullSyncStatus(userID, accountID int64) (FullSyncStatus, error) {
	account, err := s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return FullSyncStatus{}, err
	}
	return fullSyncStatusFromAccount(account), nil
}

func (s *Service) runFullSync(userID, accountID int64, cancel <-chan struct{}) {
	started := time.Now()
	defer s.unregisterFullSyncCancel(userID, accountID)
	jobID, err := s.store.CreateSyncJob(userID, accountID, "full", "running")
	if err != nil {
		log.Printf("全量同步创建任务记录失败 userID=%d accountID=%d err=%v", userID, accountID, err)
		jobID = 0
	} else {
		_ = s.store.CreateSyncJobEvent(jobID, "info", "start", "开始全量同步", mustJSON(map[string]any{
			"userId":    userID,
			"accountId": accountID,
		}))
	}
	account, err := s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		if jobID > 0 {
			_ = s.store.FinishSyncJob(jobID, "failed", 0, err.Error())
			_ = s.store.CreateSyncJobEvent(jobID, "error", "account", err.Error(), mustJSON(nil))
		}
		_ = s.store.FinishMailAccountFullSync(userID, accountID, "failed", err.Error())
		return
	}
	config, err := s.accountConfig(account)
	if err != nil {
		if jobID > 0 {
			_ = s.store.FinishSyncJob(jobID, "failed", 0, err.Error())
			_ = s.store.CreateSyncJobEvent(jobID, "error", "config", err.Error(), mustJSON(nil))
		}
		_ = s.store.FinishMailAccountFullSync(userID, accountID, "failed", err.Error())
		_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "failed", err.Error())
		return
	}

	type folderUIDs struct {
		folder string
		uids   []string
	}
	folderBatches := make([]folderUIDs, 0)
	total := 0
	for _, folder := range accountSyncFolders(account) {
		folderConfig := configForFolder(config, folder)
		uids, err := s.fetcher.ListFolderUIDs(folderConfig)
		if err != nil {
			if shouldSkipMissingOptionalFolder(folder, err) {
				log.Printf("全量同步跳过可选文件夹 userID=%d accountID=%d folder=%s", userID, accountID, folder)
				if jobID > 0 {
					_ = s.store.CreateSyncJobEvent(jobID, "warn", "folder", "文件夹 "+folder+" 不存在，已跳过", mustJSON(map[string]any{
						"folder": folder,
					}))
				}
				continue
			}
			if jobID > 0 {
				_ = s.store.FinishSyncJob(jobID, "failed", 0, err.Error())
				_ = s.store.CreateSyncJobEvent(jobID, "error", "folder", err.Error(), mustJSON(map[string]any{
					"folder": folder,
				}))
			}
			_ = s.store.FinishMailAccountFullSync(userID, accountID, "failed", err.Error())
			_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "failed", err.Error())
			return
		}
		reverseStrings(uids)
		folderBatches = append(folderBatches, folderUIDs{folder: folder, uids: uids})
		total += len(uids)
		log.Printf("全量同步目录扫描完成 userID=%d accountID=%d folder=%s uidCount=%d", userID, accountID, folder, len(uids))
	}
	_ = s.store.UpdateMailAccountFullSyncProgress(userID, accountID, total, 0, 0)
	log.Printf("全量同步开始拉取 userID=%d accountID=%d total=%d folders=%d", userID, accountID, total, len(folderBatches))

	newCount := 0
	processed := 0
	for _, folderBatch := range folderBatches {
		folderConfig := configForFolder(config, folderBatch.folder)
		for start := 0; start < len(folderBatch.uids); start += fullSyncBatchSize {
			if s.fullSyncCancelled(cancel) {
				log.Printf("全量同步已取消 userID=%d accountID=%d processed=%d total=%d new=%d", userID, accountID, processed, total, newCount)
				if jobID > 0 {
					_ = s.store.FinishSyncJob(jobID, "cancelled", newCount, "用户停止了全量同步")
					_ = s.store.CreateSyncJobEvent(jobID, "warn", "cancel", "用户停止了全量同步", mustJSON(nil))
				}
				_ = s.store.FinishMailAccountFullSync(userID, accountID, "cancelled", "用户停止了全量同步")
				return
			}
			end := start + fullSyncBatchSize
			if end > len(folderBatch.uids) {
				end = len(folderBatch.uids)
			}
			batchUIDs := folderBatch.uids[start:end]
			messages, err := s.fetcher.FetchFolderByUIDs(folderConfig, batchUIDs)
			if err != nil {
				log.Printf("全量同步批次拉取失败 userID=%d accountID=%d folder=%s processed=%d total=%d err=%v", userID, accountID, folderBatch.folder, processed, total, err)
				if jobID > 0 {
					_ = s.store.FinishSyncJob(jobID, "failed", processed, err.Error())
					_ = s.store.CreateSyncJobEvent(jobID, "error", "batch", err.Error(), mustJSON(map[string]any{
						"folder": folderBatch.folder,
					}))
				}
				_ = s.store.FinishMailAccountFullSync(userID, accountID, "failed", err.Error())
				_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "failed", err.Error())
				return
			}
			if s.fullSyncCancelled(cancel) {
				if jobID > 0 {
					_ = s.store.FinishSyncJob(jobID, "cancelled", newCount, "用户停止了全量同步")
					_ = s.store.CreateSyncJobEvent(jobID, "warn", "cancel", "用户停止了全量同步", mustJSON(nil))
				}
				_ = s.store.FinishMailAccountFullSync(userID, accountID, "cancelled", "用户停止了全量同步")
				return
			}
			for _, fetched := range messages {
				if s.fullSyncCancelled(cancel) {
					_ = s.store.FinishMailAccountFullSync(userID, accountID, "cancelled", "用户停止了全量同步")
					return
				}
				inserted, err := s.saveMessage(userID, accountID, folderBatch.folder, fetched)
				if err != nil {
					log.Printf("全量同步保存邮件失败 userID=%d accountID=%d folder=%s uid=%s processed=%d total=%d err=%v", userID, accountID, folderBatch.folder, fetched.UID, processed, total, err)
					if jobID > 0 {
						_ = s.store.FinishSyncJob(jobID, "failed", processed, err.Error())
						_ = s.store.CreateSyncJobEvent(jobID, "error", "message", err.Error(), mustJSON(map[string]any{
							"folder": folderBatch.folder,
							"uid":    fetched.UID,
						}))
					}
					_ = s.store.FinishMailAccountFullSync(userID, accountID, "failed", err.Error())
					_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "failed", err.Error())
					return
				}
				if inserted {
					newCount++
				}
			}
			processed += len(batchUIDs)
			_ = s.store.UpdateMailAccountFullSyncProgress(userID, accountID, total, processed, newCount)
			if processed == total || processed%500 == 0 {
				log.Printf("全量同步进度 userID=%d accountID=%d processed=%d total=%d new=%d", userID, accountID, processed, total, newCount)
			}
			if jobID > 0 {
				_ = s.store.CreateSyncJobEvent(jobID, "info", "batch", "批量同步完成", mustJSON(map[string]any{
					"folder":    folderBatch.folder,
					"processed": processed,
					"total":     total,
				}))
			}
		}
	}
	if s.fullSyncCancelled(cancel) {
		if jobID > 0 {
			_ = s.store.FinishSyncJob(jobID, "cancelled", newCount, "用户停止了全量同步")
			_ = s.store.CreateSyncJobEvent(jobID, "warn", "cancel", "用户停止了全量同步", mustJSON(nil))
		}
		_ = s.store.FinishMailAccountFullSync(userID, accountID, "cancelled", "用户停止了全量同步")
		return
	}

	if err := s.cleanupServerOldMessages(userID, accountID, account, config); err != nil {
		log.Printf("全量同步服务器清理失败 userID=%d accountID=%d err=%v", userID, accountID, err)
		if jobID > 0 {
			_ = s.store.FinishSyncJob(jobID, "failed", newCount, err.Error())
			_ = s.store.CreateSyncJobEvent(jobID, "warn", "cleanup", err.Error(), mustJSON(nil))
		}
		_ = s.store.FinishMailAccountFullSync(userID, accountID, "failed", err.Error())
		_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "failed", err.Error())
		return
	}

	if jobID > 0 {
		_ = s.store.FinishSyncJob(jobID, "success", newCount, "")
		_ = s.store.CreateSyncJobEvent(jobID, "info", "finish", "全量同步完成", mustJSON(map[string]any{
			"newMessageCount": newCount,
		}))
	}
	_ = s.store.FinishMailAccountFullSync(userID, accountID, "success", "")
	_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "success", "")
	log.Printf("全量同步完成 userID=%d accountID=%d total=%d processed=%d new=%d duration=%s", userID, accountID, total, processed, newCount, time.Since(started))
}

func (s *Service) registerFullSyncCancel(userID, accountID int64) chan struct{} {
	s.fullSyncMu.Lock()
	defer s.fullSyncMu.Unlock()
	key := fullSyncKey(userID, accountID)
	if existing, ok := s.fullSyncCancels[key]; ok {
		close(existing)
	}
	cancel := make(chan struct{})
	s.fullSyncCancels[key] = cancel
	return cancel
}

func (s *Service) unregisterFullSyncCancel(userID, accountID int64) {
	s.fullSyncMu.Lock()
	defer s.fullSyncMu.Unlock()
	delete(s.fullSyncCancels, fullSyncKey(userID, accountID))
}

func (s *Service) cancelFullSync(userID, accountID int64) {
	s.fullSyncMu.Lock()
	defer s.fullSyncMu.Unlock()
	key := fullSyncKey(userID, accountID)
	if cancel, ok := s.fullSyncCancels[key]; ok {
		close(cancel)
		delete(s.fullSyncCancels, key)
	}
}

func (s *Service) fullSyncCancelled(cancel <-chan struct{}) bool {
	select {
	case <-cancel:
		return true
	default:
		return false
	}
}

func fullSyncKey(userID, accountID int64) string {
	return fmt.Sprintf("%d:%d", userID, accountID)
}

func (s *Service) cleanupServerOldMessages(userID, accountID int64, account storage.MailAccount, config AccountConfig) error {
	if !account.CleanupEnabled {
		return nil
	}
	retentionDays := account.CleanupRetentionDays
	if retentionDays <= 0 {
		retentionDays = 90
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	uids, err := s.store.ListSyncedInboxUIDsBefore(userID, accountID, cutoff)
	if err != nil {
		return err
	}
	if len(uids) == 0 {
		return nil
	}
	log.Printf("全量同步后开始清理服务器旧邮件 userID=%d accountID=%d retentionDays=%d deleteCount=%d", userID, accountID, retentionDays, len(uids))
	for start := 0; start < len(uids); start += fullSyncBatchSize {
		end := start + fullSyncBatchSize
		if end > len(uids) {
			end = len(uids)
		}
		if err := s.fetcher.DeleteInboxUIDs(config, uids[start:end]); err != nil {
			return err
		}
	}
	log.Printf("全量同步后服务器旧邮件清理完成 userID=%d accountID=%d deleteCount=%d", userID, accountID, len(uids))
	return nil
}

func accountSyncFolders(account storage.MailAccount) []string {
	folders := []string{"INBOX", normalizeSentFolder(account.SentFolder)}
	seen := make(map[string]bool, len(folders))
	unique := make([]string, 0, len(folders))
	for _, folder := range folders {
		folder = normalizeFolderName(folder)
		key := strings.ToLower(folder)
		if folder == "" || seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, folder)
	}
	return unique
}

func configForFolder(config AccountConfig, folder string) AccountConfig {
	config.Folder = normalizeFolderName(folder)
	return config
}

func shouldSkipMissingOptionalFolder(folder string, err error) bool {
	if !errors.Is(err, ErrFolderNotFound) {
		return false
	}
	return !strings.EqualFold(normalizeFolderName(folder), "INBOX")
}

func fullSyncStatusFromAccount(account storage.MailAccount) FullSyncStatus {
	status := strings.TrimSpace(account.FullSyncStatus)
	if status == "" {
		status = "idle"
	}
	return FullSyncStatus{
		Status:         status,
		Total:          account.FullSyncTotal,
		Processed:      account.FullSyncProcessed,
		NewCount:       account.FullSyncNewCount,
		StartedAt:      account.FullSyncStartedAt,
		FinishedAt:     account.FullSyncFinishedAt,
		Error:          account.FullSyncError,
		CleanupEnabled: account.CleanupEnabled,
		RetentionDays:  account.CleanupRetentionDays,
	}
}

func reverseStrings(values []string) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}
