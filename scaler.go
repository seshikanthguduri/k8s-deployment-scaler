// Package main implements a minimal deployment-scaler HTTP service.
//
// The Kubernetes dependency sits behind the Scaler interface so the service
// logic, validation, and HTTP contract can be exercised with the in-memory
// implementation in this file. Swapping in client-go means writing one more
// implementation of Scaler — nothing else changes.
package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// MaxReplicas is the hard ceiling Scale will accept. Scaling is idempotent,
// so a client retry of the same request is safe.
const MaxReplicas int32 = 50

var (
	ErrNotFound = errors.New("deployment not found")
	ErrInvalid  = errors.New("invalid request")
)

// Deployment is the service's view of a Kubernetes Deployment: identity plus
// the one field this service is allowed to touch.
type Deployment struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Replicas  int32  `json:"replicas"`
}

// Scaler reads and sets replica counts. Implementations must be safe for
// concurrent use and must honor ctx cancellation.
type Scaler interface {
	GetReplicas(ctx context.Context, namespace, name string) (Deployment, error)
	Scale(ctx context.Context, namespace, name string, replicas int32) (Deployment, error)
}

func validateKey(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("%w: namespace and name are required", ErrInvalid)
	}
	return nil
}

func validateReplicas(replicas int32) error {
	if replicas < 0 || replicas > MaxReplicas {
		return fmt.Errorf("%w: replicas must be between 0 and %d", ErrInvalid, MaxReplicas)
	}
	return nil
}

// MemoryScaler is an in-memory Scaler for local runs and tests.
type MemoryScaler struct {
	mu    sync.RWMutex
	items map[string]Deployment
}

func NewMemoryScaler(seed ...Deployment) *MemoryScaler {
	m := &MemoryScaler{items: make(map[string]Deployment, len(seed))}
	for _, d := range seed {
		m.items[d.Namespace+"/"+d.Name] = d
	}
	return m
}

func (m *MemoryScaler) GetReplicas(ctx context.Context, namespace, name string) (Deployment, error) {
	if err := validateKey(namespace, name); err != nil {
		return Deployment{}, err
	}
	if err := ctx.Err(); err != nil {
		return Deployment{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.items[namespace+"/"+name]
	if !ok {
		return Deployment{}, fmt.Errorf("%w: %s/%s", ErrNotFound, namespace, name)
	}
	return d, nil
}

func (m *MemoryScaler) Scale(ctx context.Context, namespace, name string, replicas int32) (Deployment, error) {
	if err := validateKey(namespace, name); err != nil {
		return Deployment{}, err
	}
	if err := validateReplicas(replicas); err != nil {
		return Deployment{}, err
	}
	if err := ctx.Err(); err != nil {
		return Deployment{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := namespace + "/" + name
	d, ok := m.items[key]
	if !ok {
		return Deployment{}, fmt.Errorf("%w: %s/%s", ErrNotFound, namespace, name)
	}
	d.Replicas = replicas
	m.items[key] = d
	return d, nil
}
