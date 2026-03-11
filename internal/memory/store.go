package memory

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

type Store interface {
	Save(ctx context.Context, entry *model.MemoryEntry) error
	Search(ctx context.Context, agentID int64, userID, query string, limit int) ([]model.MemoryEntry, error)
}

type InMemoryStore struct {
	mu       sync.RWMutex
	memories []model.MemoryEntry
	nextID   int64
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{nextID: 1}
}

func (s *InMemoryStore) Save(_ context.Context, entry *model.MemoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry.ID = s.nextID
	s.nextID++
	entry.CreatedAt = time.Now()
	s.memories = append(s.memories, *entry)
	return nil
}

// Search 基于关键词匹配的简单检索，按 (匹配数 × 重要度) 排序。
func (s *InMemoryStore) Search(_ context.Context, agentID int64, userID, query string, limit int) ([]model.MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queryWords := strings.Fields(strings.ToLower(query))
	if len(queryWords) == 0 {
		return nil, nil
	}

	type scored struct {
		entry model.MemoryEntry
		score int
	}

	var matches []scored
	for _, m := range s.memories {
		if m.AgentID != agentID {
			continue
		}
		if userID != "" && m.UserID != "" && m.UserID != userID {
			continue
		}
		text := strings.ToLower(m.Keywords + " " + m.Content)
		score := 0
		for _, w := range queryWords {
			if strings.Contains(text, w) {
				score++
			}
		}
		if score > 0 {
			matches = append(matches, scored{entry: m, score: score * max(m.Importance, 1)})
		}
	}

	slices.SortFunc(matches, func(a, b scored) int {
		return cmp.Compare(b.score, a.score)
	})

	n := min(limit, len(matches))
	result := make([]model.MemoryEntry, n)
	for i := range n {
		result[i] = matches[i].entry
	}
	return result, nil
}
