# RFD — Deployment Scaler Service (practice build)

Status: draft · Author: S. Guduri (practice project) · Scope: learning build, not production software

## Problem

Operators occasionally need to read and adjust a Kubernetes Deployment's replica
count outside of kubectl — from automation, a chat tool, or a small internal UI.
Doing that with a full cluster-admin credential is over-privileged and hard to audit.

## Proposal

A small Go HTTP service with exactly two operations:

- `GET  /deployments/{namespace}/{name}` → current replica count as JSON
- `POST /scale` with `{"namespace","name","replicas"}` → set replica count, return the result

The service talks to the cluster through a narrow `Scaler` interface
(`GetReplicas` / `Scale`). This build ships an in-memory implementation so it
runs and tests anywhere; the production implementation would use client-go's
`AppsV1().Deployments(ns).UpdateScale()` behind the same interface.

## Least privilege (the point of the exercise)

The service account needs only replica-count access, not full Deployment write:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata: { namespace: default, name: deployment-scaler }
rules:
  - apiGroups: ["apps"]
    resources: ["deployments/scale"]
    verbs: ["get", "update", "patch"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get"]   # read spec.replicas only
```

## Validation & safety

- replicas must be 0..50 (explicit ceiling; scaling is idempotent, so retries are safe)
- namespace/name required; unknown deployment → 404, bad input → 400
- every store operation runs under the request's context (timeout/cancellation propagate)

## Alternatives considered

1. **Shell out to kubectl** — simplest, but unparseable errors, no typed
   validation, and the credential is whatever kubectl has. Rejected.
2. **Depend on client-go from day one** — realistic, but the module download
   and cluster requirement bury the Go fundamentals this build exists to teach
   (interfaces, net/http, table-driven tests). Deferred to step 2, behind the
   interface, where it is a ~40-line swap.
3. **gRPC instead of HTTP/JSON** — Teleport's own services lean gRPC, and it is
   a reasonable step 3; HTTP first keeps the contract curl-debuggable.

## Risks / open questions

- In-memory store loses state on restart — acceptable for practice; the
  client-go implementation reads truth from the API server on every call.
- No auth in this build. A real deployment sits behind the cluster's existing
  identity-aware proxy; adding a token check is listed in the README next steps.
