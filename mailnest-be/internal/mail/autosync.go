package mail

import (
	"context"
	"log"
	"time"

	"mailnest-be/internal/storage"
)

// AutoSyncOptions 控制定时收取的扫描频率、批量数量和并发上限。
type AutoSyncOptions struct {
	CheckInterval  time.Duration
	BatchLimit     int
	MaxConcurrent  int
	RunImmediately bool
}

const defaultAutoSyncCheckInterval = time.Minute

const defaultAutoSyncBatchLimit = 20

const defaultAutoSyncMaxConcurrent = 2

func (s *Service) StartAutoSyncScheduler(ctx context.Context, options AutoSyncOptions) {
	options = normalizeAutoSyncOptions(options)
	sem := make(chan struct{}, options.MaxConcurrent)
	go func() {
		log.Printf("邮件自动收取调度器已启动 interval=%s batchLimit=%d maxConcurrent=%d", options.CheckInterval, options.BatchLimit, options.MaxConcurrent)
		if options.RunImmediately {
			s.dispatchDueAutoSyncs(ctx, options, sem)
		}
		ticker := time.NewTicker(options.CheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Printf("邮件自动收取调度器已停止")
				return
			case <-ticker.C:
				s.dispatchDueAutoSyncs(ctx, options, sem)
			}
		}
	}()
}

func normalizeAutoSyncOptions(options AutoSyncOptions) AutoSyncOptions {
	if options.CheckInterval <= 0 {
		options.CheckInterval = defaultAutoSyncCheckInterval
	}
	if options.BatchLimit <= 0 || options.BatchLimit > 100 {
		options.BatchLimit = defaultAutoSyncBatchLimit
	}
	if options.MaxConcurrent <= 0 || options.MaxConcurrent > 10 {
		options.MaxConcurrent = defaultAutoSyncMaxConcurrent
	}
	return options
}

func (s *Service) dispatchDueAutoSyncs(ctx context.Context, options AutoSyncOptions, sem chan struct{}) {
	accounts, err := s.store.ListDueMailAccounts(options.BatchLimit)
	if err != nil {
		log.Printf("邮件自动收取失败：查询到期邮箱账号失败 err=%v", err)
		return
	}
	if len(accounts) > 0 {
		log.Printf("邮件自动收取扫描到到期账号 count=%d", len(accounts))
	}
	for _, account := range accounts {
		if ctx.Err() != nil {
			return
		}
		if !s.tryRegisterInboxSync(account.UserID, account.ID) {
			continue
		}
		select {
		case sem <- struct{}{}:
		default:
			s.unregisterInboxSync(account.UserID, account.ID)
			return
		}
		go func(account storage.MailAccount) {
			defer func() {
				<-sem
				s.unregisterInboxSync(account.UserID, account.ID)
			}()
			result, err := s.syncInbox(account, "auto")
			if err != nil {
				log.Printf("邮件自动收取失败 userID=%d accountID=%d err=%v", account.UserID, account.ID, err)
				return
			}
			log.Printf("邮件自动收取完成 userID=%d accountID=%d jobID=%d new=%d warnings=%d", account.UserID, account.ID, result.JobID, result.NewMessageCount, len(result.Warnings))
		}(account)
	}
}
