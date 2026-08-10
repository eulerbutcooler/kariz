package server

import (
	"sync"

	"github.com/hashicorp/yamux"
)

/*
 * NOTE: tunnel ids are like this:
 * "home"="http://localhost:8000", here ID="home" the localhost url is known by client only and never by server
 * "api"="http://localhost:3000"
 * "blog"="http://localhost:8080"
 * so sessions = {"home"->sess, "api" ->sess, "blog" ->sess}
 * and tunnels = {sess -> ["home","api","blog"]}
 */

type Registry struct {
	mu       sync.RWMutex
	sessions map[string]*yamux.Session
	tunnels  map[*yamux.Session][]string
}

func NewRegistry() *Registry {
	return &Registry{
		// NOTE: id to session mapping
		sessions: make(map[string]*yamux.Session),
		// NOTE: session -> all tunnel ids it owns
		tunnels: make(map[*yamux.Session][]string),
	}
}

func (r *Registry) Register(sess *yamux.Session, ids []string) (bound, clashes []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, id := range ids {
		if _, exists := r.sessions[id]; exists {
			clashes = append(clashes, id)
			continue
		}
		r.sessions[id] = sess
		r.tunnels[sess] = append(r.tunnels[sess], id)
		bound = append(bound, id)
	}
	return bound, clashes
}

func (r *Registry) Lookup(id string) (*yamux.Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sess, ok := r.sessions[id]
	return sess, ok
}

func (r *Registry) Unregister(sess *yamux.Session) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	ids, ok := r.tunnels[sess]
	if !ok {
		return nil
	}
	removed := append([]string(nil), ids...)
	for _, id := range removed {
		delete(r.sessions, id)
	}
	delete(r.tunnels, sess)
	return removed
}
