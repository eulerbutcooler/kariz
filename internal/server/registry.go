package server

import (
	"sync"

	"github.com/hashicorp/yamux"
)

/*
 * NOTE:
 * Two strings per tunnel: Subdomain and Label. Subdomain = Server minted,
 * Label = User specified. Ex - Subdomain = meow.cat.pizza.surang.xyz, Label = web, api, etc
 * Registry:
 * {
 * sessions={
 * "meow.cat.pizza"->Binding{sess,"web"},
 * "yo.pierre.here"->Binding{sess,"api"}
 * }
 * tunnels={
 * sess->["meow.cat.pizza", "yo.pierre.here"]
 * }
 * }
 */

type Registry struct {
	mu       sync.RWMutex
	sessions map[string]Binding
	tunnels  map[*yamux.Session][]string
}

type Binding struct {
	Session *yamux.Session
	Label   string
}

func NewRegistry() *Registry {
	return &Registry{
		// NOTE: id to session mapping
		sessions: make(map[string]Binding),
		// NOTE: session -> all tunnel ids it owns
		tunnels: make(map[*yamux.Session][]string),
	}
}

func (r *Registry) Register(sess *yamux.Session, subdomain, label string) (ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.sessions[subdomain]; exists {
		return false
	} else {
		r.sessions[subdomain] = Binding{Session: sess, Label: label}
		r.tunnels[sess] = append(r.tunnels[sess], subdomain)
		return true
	}
}

func (r *Registry) Lookup(subdomain string) (Binding, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.sessions[subdomain]
	return b, ok
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
