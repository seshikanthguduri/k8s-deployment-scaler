package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestScaleValidation(t *testing.T) {
	s := NewMemoryScaler(Deployment{Namespace: "default", Name: "web", Replicas: 3})
	cases := []struct {
		name      string
		namespace string
		dep       string
		replicas  int32
		wantErr   error
	}{
		{"happy path", "default", "web", 5, nil},
		{"scale to zero", "default", "web", 0, nil},
		{"at ceiling", "default", "web", MaxReplicas, nil},
		{"negative", "default", "web", -1, ErrInvalid},
		{"over ceiling", "default", "web", MaxReplicas + 1, ErrInvalid},
		{"missing name", "default", "", 1, ErrInvalid},
		{"unknown deployment", "default", "nope", 1, ErrNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.Scale(context.Background(), tc.namespace, tc.dep, tc.replicas)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Scale(%q,%q,%d) err = %v, want %v", tc.namespace, tc.dep, tc.replicas, err, tc.wantErr)
			}
		})
	}
}

func TestScaleThenGet(t *testing.T) {
	s := NewMemoryScaler(Deployment{Namespace: "default", Name: "web", Replicas: 3})
	if _, err := s.Scale(context.Background(), "default", "web", 7); err != nil {
		t.Fatal(err)
	}
	d, err := s.GetReplicas(context.Background(), "default", "web")
	if err != nil {
		t.Fatal(err)
	}
	if d.Replicas != 7 {
		t.Fatalf("replicas = %d, want 7", d.Replicas)
	}
}

// TestConcurrentScale hammers Scale from many goroutines; run with -race.
func TestConcurrentScale(t *testing.T) {
	s := NewMemoryScaler(Deployment{Namespace: "default", Name: "web", Replicas: 0})
	var wg sync.WaitGroup
	for i := int32(0); i < 20; i++ {
		wg.Add(1)
		go func(n int32) {
			defer wg.Done()
			if _, err := s.Scale(context.Background(), "default", "web", n%10); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
}

func TestHTTPContract(t *testing.T) {
	srv := httptest.NewServer(NewServer(NewMemoryScaler(
		Deployment{Namespace: "default", Name: "web", Replicas: 3},
	)))
	defer srv.Close()

	// GET returns the seeded replica count.
	resp, err := http.Get(srv.URL + "/deployments/default/web")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", resp.StatusCode)
	}
	var got Deployment
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Replicas != 3 {
		t.Fatalf("GET replicas = %d, want 3", got.Replicas)
	}

	// POST /scale updates it.
	resp2, err := http.Post(srv.URL+"/scale", "application/json",
		strings.NewReader(`{"namespace":"default","name":"web","replicas":9}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("POST /scale status = %d, want 200", resp2.StatusCode)
	}

	// Unknown deployment is a 404, not a 500.
	resp3, err := http.Get(srv.URL + "/deployments/default/ghost")
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Fatalf("GET unknown status = %d, want 404", resp3.StatusCode)
	}
}
