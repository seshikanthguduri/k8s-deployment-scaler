package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	// Seed one demo deployment so the service answers immediately.
	// With the client-go implementation this seed goes away — the API
	// server is the source of truth.
	scaler := NewMemoryScaler(Deployment{Namespace: "default", Name: "web", Replicas: 3})

	srv := &http.Server{
		Addr:              *addr,
		Handler:           NewServer(scaler),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("scaler listening on %s (try: curl localhost%s/deployments/default/web)", *addr, *addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve: %v", err)
		}
	}()

	// Graceful shutdown: finish in-flight requests, bounded to 10s.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
	log.Print("stopped")
}
