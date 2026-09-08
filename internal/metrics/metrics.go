package metrics

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

type Registry struct {
	mu       sync.RWMutex
	counters map[string]uint64
	gauges   map[string]int64
}

func New() *Registry {
	return &Registry{counters: make(map[string]uint64), gauges: make(map[string]int64)}
}

func sanitize(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == ':' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func (r *Registry) Inc(name string) { r.Add(name, 1) }

func (r *Registry) Add(name string, n uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters[sanitize(name)] += n
}

func (r *Registry) Set(name string, value int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gauges[sanitize(name)] = value
}

// WritePrometheus snapshots the registry before writing. HTTP clients are
// external/slow I/O and must never hold the registry lock used by the data
// plane hooks while a response is being transmitted.
func (r *Registry) WritePrometheus(w io.Writer) {
	r.mu.RLock()
	counters := make(map[string]uint64, len(r.counters))
	gauges := make(map[string]int64, len(r.gauges))
	for name, value := range r.counters {
		counters[name] = value
	}
	for name, value := range r.gauges {
		gauges[name] = value
	}
	r.mu.RUnlock()

	names := make([]string, 0, len(counters)+len(gauges))
	for name := range counters {
		names = append(names, name)
	}
	for name := range gauges {
		names = append(names, name)
	}
	sort.Strings(names)

	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		if value, ok := counters[name]; ok {
			_, _ = fmt.Fprintf(w, "%s %d\n", name, value)
		}
		if value, ok := gauges[name]; ok {
			_, _ = fmt.Fprintf(w, "%s %d\n", name, value)
		}
	}
}
