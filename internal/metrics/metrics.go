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
func (r *Registry) WritePrometheus(w io.Writer) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.counters)+len(r.gauges))
	for n := range r.counters {
		names = append(names, n)
	}
	for n := range r.gauges {
		names = append(names, n)
	}
	sort.Strings(names)
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			continue
		}
		seen[n] = true
		if v, ok := r.counters[n]; ok {
			fmt.Fprintf(w, "%s %d\n", n, v)
		}
		if v, ok := r.gauges[n]; ok {
			fmt.Fprintf(w, "%s %d\n", n, v)
		}
	}
}
