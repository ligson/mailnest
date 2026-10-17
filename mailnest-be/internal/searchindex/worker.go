package searchindex

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/blevesearch/bleve/v2"
	"mailnest-be/internal/storage"
)

func (s *Service) run(ctx context.Context) {
	for ctx.Err() == nil {
		worked, err := s.step(ctx)
		s.recordError(err)
		delay := time.Second
		if worked {
			delay = time.Duration(s.cfg.BatchDelayMS) * time.Millisecond
		}
		if err != nil {
			delay = 3 * time.Second
		}
		if !wait(ctx, delay) {
			return
		}
	}
}

// 一次只处理一个小批，历史回填与增量任务交替进行，避免重建饿死新邮件。
func (s *Service) step(ctx context.Context) (bool, error) {
	release, err := s.store.Workload().Background(ctx, "index")
	if err != nil {
		return false, err
	}
	defer release()
	s.workMu.Lock()
	defer s.workMu.Unlock()
	s.mu.RLock()
	requested := s.requested
	s.mu.RUnlock()
	if requested {
		if err := s.beginBuild(ctx); err != nil {
			return false, err
		}
	}
	tasks, err := s.store.ListMailSearchTasks(ctx, s.cfg.BatchSize)
	if err != nil {
		return false, err
	}
	if len(tasks) > 0 {
		if err = s.applyTasks(ctx, tasks); err != nil {
			_ = s.store.RetryMailSearchTasks(ctx, tasks)
			return false, err
		}
	}
	s.mu.RLock()
	building := s.building
	cp := s.progress
	s.mu.RUnlock()
	if building == nil {
		return len(tasks) > 0, nil
	}
	docs, err := s.store.ListMailSearchDocuments(ctx, cp.Cursor, cp.MaxID, s.cfg.BatchSize)
	if err != nil {
		return false, err
	}
	if len(docs) > 0 {
		batch := building.NewBatch()
		for _, d := range docs {
			if ctx.Err() != nil {
				return false, ctx.Err()
			}
			if err = batch.Index(documentKey(d.UserID, d.ID), indexDocument(d)); err != nil {
				return false, err
			}
			cp.Cursor = d.ID
		}
		raw, _ := json.Marshal(cp)
		batch.SetInternal(checkpointKey, raw)
		if err = building.Batch(batch); err != nil {
			return false, err
		}
		s.mu.Lock()
		s.progress = cp
		s.mu.Unlock()
		return true, nil
	}
	// 尚有退避中的失败任务时不发布不完整副本；旧索引仍可正常搜索。
	stats, err := s.store.MailSearchTasksStats(ctx, 0)
	if err != nil {
		return false, err
	}
	if stats.Pending > 0 {
		return len(tasks) > 0, nil
	}
	if err = s.publish(); err != nil {
		return false, err
	}
	return true, nil
}
func (s *Service) beginBuild(ctx context.Context) error {
	maxID, err := s.store.MaxMailSearchMessageID(ctx)
	if err != nil {
		return err
	}
	name := fmt.Sprintf("index-v1-%d", time.Now().UnixNano())
	index, err := bleve.New(filepath.Join(s.path, name), indexMapping())
	if err != nil {
		return err
	}
	cp := checkpoint{MaxID: maxID}
	raw, _ := json.Marshal(cp)
	if err = index.SetInternal(checkpointKey, raw); err != nil {
		_ = index.Close()
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.manifest
	next.Building = name
	if err = s.saveManifest(next); err != nil {
		_ = index.Close()
		return err
	}
	s.manifest = next
	s.building = index
	s.progress = cp
	s.requested = false
	log.Printf("开始邮件搜索索引重建 generation=%s max_id=%d batch_size=%d", name, maxID, s.cfg.BatchSize)
	return nil
}
func (s *Service) applyTasks(ctx context.Context, tasks []storage.MailSearchTask) error {
	docs, err := s.store.MailSearchDocumentsByTasks(ctx, tasks)
	if err != nil {
		return err
	}
	byID := make(map[int64]storage.MailSearchDocument, len(docs))
	for _, d := range docs {
		byID[d.ID] = d
	}
	// workMu 防止构建/发布与写入交错；索引批提交后才确认精确事件 ID。
	s.mu.RLock()
	indexes := []bleve.Index{s.active, s.building}
	s.mu.RUnlock()
	for _, index := range indexes {
		if index == nil {
			continue
		}
		batch := index.NewBatch()
		for _, task := range tasks {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			d, ok := byID[task.MessageID]
			if !ok || d.UserID != task.UserID {
				batch.Delete(documentKey(task.UserID, task.MessageID))
			}
			if ok {
				if err = batch.Index(documentKey(d.UserID, d.ID), indexDocument(d)); err != nil {
					return err
				}
			}
		}
		if err = index.Batch(batch); err != nil {
			return err
		}
	}
	return s.store.CompleteMailSearchTasks(ctx, tasks)
}
func (s *Service) publish() error {
	s.mu.Lock()
	next := s.manifest
	obsolete := next.Previous
	if next.Active != "" {
		next.Previous = next.Active
	}
	next.Active = next.Building
	next.Building = ""
	if err := s.saveManifest(next); err != nil {
		s.mu.Unlock()
		return err
	}
	old := s.active
	s.active = s.building
	s.building = nil
	s.manifest = next
	s.lastError = ""
	// 等旧读者退出并切换指针后即可放行新搜索；旧索引关闭/清理可能较慢。
	s.mu.Unlock()
	if old != nil {
		if err := old.Close(); err != nil {
			log.Printf("关闭上一代搜索索引失败 err=%v", err)
		}
	}
	log.Printf("邮件搜索索引重建完成 generation=%s", next.Active)
	// 仅清理清单中曾明确记录的更早代际；不遍历删除未知目录或用户数据。
	if obsolete != "" && obsolete != next.Active && obsolete != next.Previous && validGeneration(obsolete) {
		if err := os.RemoveAll(filepath.Join(s.path, obsolete)); err != nil {
			log.Printf("清理旧搜索索引失败 generation=%s err=%v", obsolete, err)
		}
	}
	return nil
}
