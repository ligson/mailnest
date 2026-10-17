package searchindex

import "context"

type Status struct {
	Engine        string `json:"engine"`
	Mode          string `json:"mode"`
	Available     bool   `json:"available"`
	Phase         string `json:"phase"`
	Done          int64  `json:"done"`
	Total         int64  `json:"total"`
	Pending       int64  `json:"pending"`
	Failed        int64  `json:"failed"`
	Message       string `json:"message"`
	ShadowChecks  *int64 `json:"shadowChecks,omitempty"`
	ShadowMissing *int64 `json:"shadowMissing,omitempty"`
}

func (s *Service) Status(ctx context.Context, userID int64, admin bool) (Status, error) {
	s.mu.RLock()
	cp := s.progress
	building := s.building != nil || s.requested
	result := Status{Engine: "bleve", Mode: s.cfg.Mode, Available: s.active != nil && !s.closed, Phase: "ready", Message: s.lastError}
	s.mu.RUnlock()
	if building {
		result.Phase = "building"
	} else {
		cp.Cursor = 1<<63 - 1
		cp.MaxID = cp.Cursor
	}
	done, total, err := s.store.MailSearchSourceProgress(ctx, userID, cp.Cursor, cp.MaxID)
	if err != nil {
		return result, err
	}
	stats, err := s.store.MailSearchTasksStats(ctx, userID)
	if err != nil {
		return result, err
	}
	result.Done, result.Total, result.Pending, result.Failed = done, total, stats.Pending, stats.Failed
	// 已追平后不继续展示历史失败；只提示当前用户仍在重试的任务。
	if !building && result.Available && stats.Pending == 0 {
		result.Message = ""
	}
	if stats.Failed > 0 && result.Message == "" {
		result.Message = "部分索引任务正在自动重试，邮件仍保存在数据库中"
	}
	if admin {
		checks, missing := s.shadowChecks.Load(), s.shadowMissing.Load()
		result.ShadowChecks = &checks
		result.ShadowMissing = &missing
	}
	return result, nil
}
