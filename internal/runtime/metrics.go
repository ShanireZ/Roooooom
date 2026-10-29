package runtime

import (
	"sort"
	"sync"
	"time"
)

// Metrics keeps a bounded window with fixed labels. Callers cannot record
// project IDs, arbitrary command names, request bodies or credential material.
type Metrics struct {
	mu                 sync.Mutex
	commands, rejected uint64
	recent             [256]time.Duration
	next, size         int
}
type MetricsSnapshot struct {
	Commands         uint64  `json:"commands"`
	Rejected         uint64  `json:"rejected"`
	CommandP95Millis float64 `json:"commandP95Millis"`
	Window           int     `json:"window"`
}

func (m *Metrics) Command(d time.Duration, failed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commands++
	if failed {
		m.rejected++
	}
	m.recent[m.next] = d
	m.next = (m.next + 1) % len(m.recent)
	if m.size < len(m.recent) {
		m.size++
	}
}
func (m *Metrics) Snapshot() MetricsSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := MetricsSnapshot{Commands: m.commands, Rejected: m.rejected, Window: m.size}
	if m.size > 0 {
		values := append([]time.Duration{}, m.recent[:m.size]...)
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		result.CommandP95Millis = float64(values[(95*len(values)+99)/100-1]) / float64(time.Millisecond)
	}
	return result
}
