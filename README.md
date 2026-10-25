# k8s-deployment-scaler

A miniature Kubernetes deployment scaler in Go: an HTTP service that reads a
Deployment's replica count and scales it — with permissions scoped to replica
counts only (least-privilege by design).

Design first: see [`RFD.md`](RFD.md) for the design doc — problem, RBAC draft,
alternatives considered, and risks.

## Run it

```sh
go run . -addr :8080
curl localhost:8080/deployments/default/web
curl -X POST localhost:8080/scale -d '{"namespace":"default","name":"web","replicas":5}'
```

It seeds one in-memory deployment (`default/web`, 3 replicas) so the HTTP
contract works with no cluster. Ctrl-C shuts down gracefully.

## Verified

```sh
go vet ./...          # clean
go test -race ./...   # passes, race-detector clean (Go 1.23)
```

## Layout

- `RFD.md` — design doc: problem, least-privilege RBAC draft, alternatives, risks
- `scaler.go` — the `Scaler` interface, validation (0..50 replicas), in-memory implementation
- `server.go` — HTTP contract (Go 1.22 mux patterns), JSON errors: 400 / 404 / 500, 5s per-request context
- `main.go` — flags, server lifecycle, graceful shutdown on SIGINT/SIGTERM
- `scaler_test.go` — table-driven validation tests, a 20-goroutine concurrency test for `-race`, full HTTP contract test via httptest

## Design choices

- **Interface before client-go.** All cluster access sits behind the `Scaler`
  interface. The real implementation (`AppsV1().Deployments(ns).UpdateScale()`)
  is a small second implementation; service logic and tests don't change. It
  also keeps this build dependency-free, so `go test` works offline.
- **Least privilege.** The service can change replica counts and nothing else —
  the RBAC in the RFD grants only the `deployments/scale` subresource.

## Next steps

1. Implement the client-go backed `Scaler` behind a `-kubeconfig` flag.
2. Add Prometheus metrics for scale operations.
3. Helm chart with the scoped RBAC from the RFD.

---
A Go learning project by Seshikanth Guduri — Senior DevOps Platform SRE
(Kubernetes CKA/CKS). Production automation background: Python/Bash; Go is my
current build-in-public focus.
