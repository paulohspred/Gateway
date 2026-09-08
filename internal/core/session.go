package core

import (
	"sort"
	"sync"
	"time"
)

type Session struct {
	ID                   string    `json:"id"`
	ListenerID           string    `json:"listenerId"`
	Transport            string    `json:"transport"`
	RemoteAddr           string    `json:"remoteAddr"`
	LocalAddr            string    `json:"localAddr"`
	OpenedAt             time.Time `json:"openedAt"`
	LastSeenAt           time.Time `json:"lastSeenAt"`
	BytesFieldToConsumer uint64    `json:"bytesFieldToConsumer"`
	BytesConsumerToField uint64    `json:"bytesConsumerToField"`
}

type SessionRegistry struct {
	mu       sync.RWMutex
	sessions map[string]Session
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{sessions: make(map[string]Session)}
}

func (r *SessionRegistry) Open(s Session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[s.ID] = s
}

func (r *SessionRegistry) Touch(id, direction string, bytes int, at time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if !ok {
		return
	}
	s.LastSeenAt = at
	if bytes > 0 {
		if direction == "field_to_consumer" {
			s.BytesFieldToConsumer += uint64(bytes)
		} else if direction == "consumer_to_field" {
			s.BytesConsumerToField += uint64(bytes)
		}
	}
	r.sessions[id] = s
}

func (r *SessionRegistry) Close(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, id)
}

func (r *SessionRegistry) Snapshot() []Session {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *SessionRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}
