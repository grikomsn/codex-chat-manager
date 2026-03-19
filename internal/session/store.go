package session

import (
	"sync"
	"time"
)

const cacheTTL = 5 * time.Second

type cachedSnapshot struct {
	snapshot Snapshot
	cachedAt time.Time
}

type Store struct {
	cfg     Config
	cacheMu sync.RWMutex
	cache   *cachedSnapshot
}

func NewStore(cfg Config) *Store {
	return &Store{cfg: cfg}
}

func (s *Store) Config() Config {
	return s.cfg
}

func (s *Store) LoadSnapshot() (Snapshot, error) {
	s.cacheMu.RLock()
	if s.cache != nil && time.Since(s.cache.cachedAt) < cacheTTL {
		snapshot := s.cache.snapshot
		s.cacheMu.RUnlock()
		return snapshot, nil
	}
	s.cacheMu.RUnlock()

	snapshot, err := s.loadSnapshotFresh()
	if err != nil {
		return Snapshot{}, err
	}

	s.cacheMu.Lock()
	s.cache = &cachedSnapshot{
		snapshot: snapshot,
		cachedAt: time.Now(),
	}
	s.cacheMu.Unlock()

	return snapshot, nil
}

func (s *Store) loadSnapshotFresh() (Snapshot, error) {
	return loadSnapshotImpl(s.cfg)
}

func (s *Store) InvalidateCache() {
	s.cacheMu.Lock()
	s.cache = nil
	s.cacheMu.Unlock()
}
