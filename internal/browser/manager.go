package browser

import "sync"

// Manager tracks the browser worker's state. It deliberately has no cookie or password access.
type Manager struct {
	mu                  sync.RWMutex
	connected, loggedIn bool
	reason              string
}

func (m *Manager) Report(connected, loggedIn bool, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected, m.loggedIn, m.reason = connected, loggedIn, reason
}
func (m *Manager) Status() (bool, bool, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected, m.loggedIn, m.reason
}
