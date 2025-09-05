# Metamorph Service Contracts (v0.1)

This repository contains protobuf contracts and Kubernetes manifests for a Teranode-class, microservices-based BSV node.

## Structure

```
proto/                 # .proto contracts for Ledger, Engine, Events
k8s/manifests/         # Namespaces, Deployments, StatefulSets, HPA, KEDA
Makefile               # `make proto` using buf
buf.gen.yaml           # buf generate configuration
```

## Getting started

1. Install `buf` and language toolchains (Go/Rust/Python as needed).
2. Generate code:
   ```bash
   make proto
   ```
3. Apply namespaces and sample workloads:
   ```bash
   kubectl apply -f k8s/manifests/namespaces.yaml
   kubectl apply -f k8s/manifests/verifier-deploy.yaml
   kubectl apply -f k8s/manifests/ledger-statefulset.yaml
   kubectl apply -f k8s/manifests/verifier-hpa.yaml
   kubectl apply -f k8s/manifests/verifier-keda.yaml
   ```

> Replace `yourrepo/*` images and configure your message bus (Redpanda/NATS JetStream) and datastore (Aerospike/Scylla).

## Services covered

- **Ledger (gRPC)** — Authoritative UTXO read/apply.
- **Engine (gRPC)** — Script execution with resource limits.
- **Events** — Shared envelopes/payloads for the durable message bus.

SLO targets (non-binding):
- `Ledger.GetOutputs`: p99 ≤ 1 ms (batch ≤ 10 inputs)
- `Engine.Execute`: p99 ≤ 10 ms for standard script paths
- End-to-end `Portal → tx.ready`: p99 ≤ 300 ms (single-ancestor tx)

## License

MIT (or your preferred).

