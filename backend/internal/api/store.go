package api

import (
	"errors"
	"sort"
	"strings"
	"sync"
)

var errNotFound = errors.New("not found")

type memStore struct {
    mu   sync.RWMutex
    items []Container
}

func newMemStore() *memStore {
    return &memStore{items: demoData()}
}

func (m *memStore) snapshot() []Container {
    m.mu.RLock()
    defer m.mu.RUnlock()
    out := make([]Container, len(m.items))
    copy(out, m.items)
    sort.Slice(out, func(i, j int) bool {
        ri := out[i].Status == "running"
        rj := out[j].Status == "running"
        if ri == rj {
            return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
        }
        return ri && !rj
    })
    return out
}

func (m *memStore) setStatus(id, status string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    for i := range m.items {
        if m.items[i].ID == id {
            m.items[i].Status = status
            return nil
        }
    }
    return errNotFound
}

func (m *memStore) exists(id string) bool {
    m.mu.RLock()
    defer m.mu.RUnlock()
    for i := range m.items {
        if m.items[i].ID == id {
            return true
        }
    }
    return false
}

func demoData() []Container {
    return []Container{
        {
            ID:     "demo-u1",
            Name:   "u1",
            Image:  "ubuntu:24.04",
            Status: "running",
            Labels: map[string]string{},
            Ports:  []Port{{PublicPort: 10022, PrivatePort: 22, Type: "tcp"}},
        },
        {
            ID:     "demo-u2",
            Name:   "u2",
            Image:  "ubuntu:24.04",
            Status: "exited",
            Labels: map[string]string{},
        },
        {
            ID:     "demo-alpine",
            Name:   "alpine",
            Image:  "alpine:3.20",
            Status: "running",
            Labels: map[string]string{},
        },
    }
}
