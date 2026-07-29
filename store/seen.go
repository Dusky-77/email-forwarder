package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// each account gets its own small json file with the last historyId we processed
// gmail history ids only ever go up so this is enough to know whats new

type accountState struct {
	LastHistoryID uint64 `json:"last_history_id"`
}

type Store struct {
	dir string
	mu  sync.Mutex
}

func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

func (s *Store) path(accountName string) string {
	return filepath.Join(s.dir, accountName+".json")
}

// GetLastHistoryID returns 0 if we have never checked this account before
func (s *Store) GetLastHistoryID(accountName string) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path(accountName))
	if err != nil {
		// no file yet, first run for this account
		return 0
	}

	var state accountState
	if err := json.Unmarshal(data, &state); err != nil {
		return 0
	}

	return state.LastHistoryID
}

func (s *Store) SetLastHistoryID(accountName string, historyID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := accountState{LastHistoryID: historyID}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path(accountName), data, 0644)
}
