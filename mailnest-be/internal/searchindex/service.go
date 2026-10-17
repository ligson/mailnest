package searchindex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/gofrs/flock"
	"mailnest-be/internal/config"
	"mailnest-be/internal/storage"
)

var ErrBusy = errors.New("索引正在重建，请稍后再试")
var checkpointKey = []byte("mailnest-checkpoint-v1")

type manifest struct {
	Version  int    `json:"version"`
	Active   string `json:"active,omitempty"`
	Building string `json:"building,omitempty"`
	Previous string `json:"previous,omitempty"`
}
type checkpoint struct {
	Cursor int64 `json:"cursor"`
	MaxID  int64 `json:"maxId"`
}

// Service 仅支持一个写进程；读请求与代际切换通过 mu 协调，批处理由 workMu 串行化。
type Service struct {
	store            *storage.Store
	cfg              config.SearchConfig
	path             string
	lock             *flock.Flock
	workMu           sync.Mutex
	mu               sync.RWMutex
	manifest         manifest
	active, building bleve.Index
	progress         checkpoint
	requested        bool
	lastError        string
	cancel           context.CancelFunc
	done             chan struct{}
	started, closed  bool
	shadowChecks     atomic.Int64
	shadowMissing    atomic.Int64
}

func Open(store *storage.Store, cfg config.SearchConfig, dataDir string) (*Service, error) {
	if cfg.Mode == "" {
		cfg.Mode = "shadow"
	}
	if cfg.Mode != "shadow" && cfg.Mode != "active" {
		return nil, fmt.Errorf("search.mode 仅支持 shadow 或 active")
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 25
	}
	if cfg.BatchSize > 200 {
		cfg.BatchSize = 200
	}
	if cfg.BatchDelayMS < 10 {
		cfg.BatchDelayMS = 50
	}
	if cfg.MaxCandidates <= 0 {
		cfg.MaxCandidates = 20000
	}
	if cfg.MaxCandidates > 50000 {
		cfg.MaxCandidates = 50000
	}
	path := cfg.Path
	if path == "" {
		path = filepath.Join(dataDir, "search")
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(path, "writer.lock"))
	locked, err := lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, fmt.Errorf("索引目录已被其他进程占用")
	}
	_ = os.Chmod(filepath.Join(path, "writer.lock"), 0600)
	s := &Service{store: store, cfg: cfg, path: path, lock: lock, manifest: manifest{Version: formatVersion}, done: make(chan struct{})}
	if err := s.restore(); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}
func validGeneration(name string) bool {
	return name == "" || filepath.Base(name) == name && strings.HasPrefix(name, "index-v1-") && !strings.ContainsAny(name, `/\`)
}
func (s *Service) restore() error {
	raw, err := os.ReadFile(filepath.Join(s.path, "manifest.json"))
	if errors.Is(err, os.ErrNotExist) {
		s.requested = true
		return nil
	}
	if err != nil {
		return err
	}
	if err = json.Unmarshal(raw, &s.manifest); err != nil {
		return fmt.Errorf("读取索引清单失败：%w", err)
	}
	if s.manifest.Version != formatVersion || !validGeneration(s.manifest.Active) || !validGeneration(s.manifest.Building) || !validGeneration(s.manifest.Previous) {
		return fmt.Errorf("索引清单版本或目录名不兼容，请使用独立新目录重建")
	}
	if s.manifest.Active != "" {
		opened, openErr := bleve.Open(filepath.Join(s.path, s.manifest.Active))
		err = openErr
		if err != nil {
			log.Printf("邮件索引无法打开，保留原文件并重建 err=%v", err)
			s.lastError = "索引不可用，正在重建；搜索暂用数据库"
			s.manifest.Previous = s.manifest.Active
			s.manifest.Active = ""
		} else {
			s.active = opened
		}
	}
	if s.manifest.Building != "" {
		opened, openErr := bleve.Open(filepath.Join(s.path, s.manifest.Building))
		err = openErr
		if err == nil {
			s.building = opened
			var cp []byte
			cp, err = s.building.GetInternal(checkpointKey)
			if err == nil {
				err = json.Unmarshal(cp, &s.progress)
			}
			if err == nil && (s.progress.Cursor < 0 || s.progress.MaxID < s.progress.Cursor) {
				err = fmt.Errorf("索引检查点不合法")
			}
		}
		if err != nil {
			if s.building != nil {
				_ = s.building.Close()
			}
			s.building = nil
			s.manifest.Building = ""
			log.Printf("索引检查点无法恢复，将建立新代际 err=%v", err)
		}
	}
	s.requested = s.building == nil && s.active == nil
	return nil
}

// 清单先 fsync 再原子替换，崩溃后只会看到完整的旧清单或新清单。
func (s *Service) saveManifest(m manifest) error {
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.path, ".manifest-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(raw); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), filepath.Join(s.path, "manifest.json")); err != nil {
		return err
	}
	dir, err := os.Open(s.path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func (s *Service) Start(parent context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started || s.closed {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.started = true
	go func() { defer close(s.done); s.run(ctx) }()
}
func (s *Service) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	started := s.started
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	if started {
		<-s.done
	}
	s.workMu.Lock()
	defer s.workMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	var errs []error
	if s.active != nil {
		errs = append(errs, s.active.Close())
		s.active = nil
	}
	if s.building != nil {
		errs = append(errs, s.building.Close())
		s.building = nil
	}
	errs = append(errs, s.lock.Unlock())
	return errors.Join(errs...)
}
func (s *Service) RequestRebuild() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("索引服务已关闭")
	}
	if s.requested || s.building != nil {
		return ErrBusy
	}
	s.requested = true
	return nil
}
func (s *Service) Mode() string { return "bleve-" + s.cfg.Mode }
func (s *Service) recordError(err error) {
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}
	s.mu.Lock()
	s.lastError = "索引处理暂时失败，任务已保留并将自动重试"
	s.mu.Unlock()
	log.Printf("邮件索引处理失败，保留任务重试 err=%v", err)
}
func wait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
