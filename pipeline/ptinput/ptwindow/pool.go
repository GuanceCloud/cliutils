package ptwindow

import (
	"sync"

	"github.com/GuanceCloud/cliutils/pkg/hash"
)

type WindowPool struct {
	sync.RWMutex
	pool map[[2]uint64]*PtWindow
}

func (m *WindowPool) Register(before, after int, k, v []string) {
	m.Lock()
	defer m.Unlock()

	key := [2]uint64{
		hash.Fnv1aHash(k),
		hash.Fnv1aHash(v),
	}

	if m.pool == nil {
		m.pool = make(map[[2]uint64]*PtWindow)
	}

	_, ok := m.pool[key]
	if !ok {
		m.pool[key] = NewWindow(before, after)
	}
}

func (m *WindowPool) Get(k, v []string) (*PtWindow, bool) {
	m.RLock()
	defer m.RUnlock()

	key := [2]uint64{
		hash.Fnv1aHash(k),
		hash.Fnv1aHash(v),
	}
	w, ok := m.pool[key]
	return w, ok
}

func NewManager() *WindowPool {
	return &WindowPool{
		pool: make(map[[2]uint64]*PtWindow),
	}
}
