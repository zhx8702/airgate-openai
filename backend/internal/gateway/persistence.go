package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

const (
	codexUsageSnapshotTable         = "plugin_account_usage_snapshots"
	codexUsagePersistenceFlushEvery = 5 * time.Second
	codexUsagePersistenceQueueSize  = 1024
)

type codexUsagePersistenceStore struct {
	db       *sql.DB
	logger   *slog.Logger
	pluginID string

	flushCh chan int64
	stopCh  chan struct{}

	pending sync.Map // accountID -> *CodexUsageSnapshot
	wg      sync.WaitGroup
}

func newCodexUsagePersistenceStore(dsn, pluginID string, logger *slog.Logger) (*codexUsagePersistenceStore, error) {
	if logger == nil {
		logger = slog.Default()
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open snapshot db: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping snapshot db: %w", err)
	}

	store := &codexUsagePersistenceStore{
		db:       db,
		logger:   logger,
		pluginID: pluginID,
		flushCh:  make(chan int64, codexUsagePersistenceQueueSize),
		stopCh:   make(chan struct{}),
	}
	if err := store.ensureSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	store.wg.Add(1)
	go store.run()
	return store, nil
}

func (s *codexUsagePersistenceStore) ensureSchema(ctx context.Context) error {
	query := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
  plugin_id  text        NOT NULL,
  account_id bigint      NOT NULL,
  snapshot   jsonb       NOT NULL,
  captured_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT NOW(),
  created_at timestamptz NOT NULL DEFAULT NOW(),
  PRIMARY KEY (plugin_id, account_id)
);
CREATE INDEX IF NOT EXISTS idx_%s_plugin_updated_at
  ON %s (plugin_id, updated_at DESC);`,
		codexUsageSnapshotTable,
		codexUsageSnapshotTable,
		codexUsageSnapshotTable,
	)

	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("ensure snapshot schema: %w", err)
	}
	return nil
}

func (s *codexUsagePersistenceStore) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(codexUsagePersistenceFlushEvery)
	defer ticker.Stop()

	for {
		select {
		case accountID := <-s.flushCh:
			s.flushAccount(context.Background(), accountID)
		case <-ticker.C:
			s.flushAll(context.Background())
		case <-s.stopCh:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			s.flushAll(ctx)
			cancel()
			return
		}
	}
}

func (s *codexUsagePersistenceStore) SaveAsync(accountID int64, snapshot *CodexUsageSnapshot) {
	if s == nil || snapshot == nil {
		return
	}
	cloned := cloneCodexUsageSnapshot(snapshot)
	s.pending.Store(accountID, cloned)

	select {
	case s.flushCh <- accountID:
	default:
		// 队列满时保留 pending 快照，交给定时 flush 合并写入。
	}
}

func (s *codexUsagePersistenceStore) flushAll(ctx context.Context) {
	if s == nil {
		return
	}
	s.pending.Range(func(key, _ any) bool {
		accountID, ok := key.(int64)
		if ok {
			s.flushAccount(ctx, accountID)
		}
		return true
	})
}

func (s *codexUsagePersistenceStore) flushAccount(ctx context.Context, accountID int64) {
	if s == nil {
		return
	}
	val, ok := s.pending.Load(accountID)
	if !ok {
		return
	}
	snapshot, ok := val.(*CodexUsageSnapshot)
	if !ok || snapshot == nil {
		s.pending.Delete(accountID)
		return
	}
	if err := s.upsert(ctx, accountID, snapshot); err != nil {
		s.logger.Warn("持久化 Codex 用量快照失败", "account_id", accountID, "error", err)
		return
	}
	s.pending.Delete(accountID)
}

func (s *codexUsagePersistenceStore) upsert(ctx context.Context, accountID int64, snapshot *CodexUsageSnapshot) error {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	capturedAt := snapshot.CapturedAt.UTC()
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}

	query := fmt.Sprintf(`
INSERT INTO %s (plugin_id, account_id, snapshot, captured_at, updated_at)
VALUES ($1, $2, $3::jsonb, $4, NOW())
ON CONFLICT (plugin_id, account_id)
DO UPDATE SET
  snapshot = EXCLUDED.snapshot,
  captured_at = EXCLUDED.captured_at,
  updated_at = NOW()`, codexUsageSnapshotTable)

	if _, err := s.db.ExecContext(ctx, query, s.pluginID, accountID, string(payload), capturedAt); err != nil {
		return fmt.Errorf("upsert snapshot: %w", err)
	}
	return nil
}

func (s *codexUsagePersistenceStore) Load(ctx context.Context, accountID int64) (*CodexUsageSnapshot, error) {
	if s == nil {
		return nil, nil
	}
	query := fmt.Sprintf(`SELECT snapshot FROM %s WHERE plugin_id = $1 AND account_id = $2`, codexUsageSnapshotTable)
	var payload []byte
	err := s.db.QueryRowContext(ctx, query, s.pluginID, accountID).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load snapshot: %w", err)
	}

	var snapshot CodexUsageSnapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot: %w", err)
	}
	return &snapshot, nil
}

func (s *codexUsagePersistenceStore) WarmCache(ctx context.Context) error {
	if s == nil {
		return nil
	}
	query := fmt.Sprintf(`SELECT account_id, snapshot FROM %s WHERE plugin_id = $1`, codexUsageSnapshotTable)
	rows, err := s.db.QueryContext(ctx, query, s.pluginID)
	if err != nil {
		return fmt.Errorf("warm cache query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	count := 0
	for rows.Next() {
		var accountID int64
		var payload []byte
		if err := rows.Scan(&accountID, &payload); err != nil {
			return fmt.Errorf("warm cache scan: %w", err)
		}

		var snapshot CodexUsageSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			s.logger.Warn("跳过损坏的 Codex 用量快照", "account_id", accountID, "error", err)
			continue
		}
		usageStore.Store(accountID, &snapshot)
		count++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("warm cache rows: %w", err)
	}

	if count > 0 {
		s.logger.Info("已预热 Codex 用量快照缓存", "count", count)
	}
	return nil
}

func (s *codexUsagePersistenceStore) Close() error {
	if s == nil {
		return nil
	}
	close(s.stopCh)
	s.wg.Wait()
	return s.db.Close()
}

func cloneCodexUsageSnapshot(snapshot *CodexUsageSnapshot) *CodexUsageSnapshot {
	if snapshot == nil {
		return nil
	}
	cloned := *snapshot
	if cloned.CapturedAt.IsZero() {
		cloned.CapturedAt = time.Now().UTC()
	}
	return &cloned
}

var (
	codexUsagePersistenceMu sync.RWMutex
	codexUsagePersistence   *codexUsagePersistenceStore
)

func setCodexUsagePersistenceStore(store *codexUsagePersistenceStore) {
	codexUsagePersistenceMu.Lock()
	defer codexUsagePersistenceMu.Unlock()
	codexUsagePersistence = store
}

func getCodexUsagePersistenceStore() *codexUsagePersistenceStore {
	codexUsagePersistenceMu.RLock()
	defer codexUsagePersistenceMu.RUnlock()
	return codexUsagePersistence
}
