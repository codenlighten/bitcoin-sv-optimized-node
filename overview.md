Awesome—let’s sketch a battle-tested microservices layout for a Teranode-class BSV node. I’ll give you the service map, data/IPC contracts, hot-path SLOs, and tech picks so an engineering team can build from this directly.

# Core service map (single-responsibility, horizontally scalable)

1. P2P Ingress/Egress — **Sentinel**
   All peer discovery + inv/tx/block gossip. Absolutely no consensus validation here; checksum + framing only. Publishes raw tx/block events; consumes broadcast intents. (Keeps DoS away from hot compute.) ([BSV Blockchain][1])

2. Stateless TX Validator — **Verifier**
   Syntax, size, standardness, and signature checks only. Purely stateless; scale out to thousands of pods. Emits `tx.validated` or `tx.validation_failed`. (This mirrors BSV’s parallel validation, but as a distributed pool.) ([BSV Association][2])

3. UTXO Lookup/Apply — **Ledger**
   Authoritative UTXO set with ultra-low-latency reads and atomic block diffs. Exposes synchronous gRPC (`GetOutputs`, `ApplyBlock`). Backed by Aerospike or ScyllaDB for millions of ops/sec at sub-ms p99. ([Aerospike][3], [BSV Blockchain][4])

4. Script Execution Sandbox — **Engine**
   Per-input script execution in a resource-capped sandbox (e.g., WASM/jailer + cgroups). Stateless; consumes `tx.exec_request`, emits `tx.exec_result`. (Keep consensus script rules in Rust; isolate resource abuse.) ([BSV Blockchain][1])

5. TX Orchestrator (mempool replacement) — **Conductor**
   Owns unconfirmed-tx lifecycle, dependency graph, and package/ancestor fee policy. Makes synchronous UTXO calls to **Ledger**; fans out script checks to **Engine**; emits `tx.ready`. Durable store (KV or graph) so “mempool” isn’t RAM-fragile. (Directly addresses historical “evicted transactions” pain.) ([CoinGeek][5])

6. Block Builder — **Architect**
   Consumes `tx.ready` (fee-prioritized queues) and constructs templates (Merkle, coinbase). Hooks to pool/stratum to feed ASICs. Miners can plug in custom selection policy. ([CoinGeek][6])

7. Chain & Consensus — **Chronicler**
   Validates PoW + coordinates full block validation with **Ledger**, commits canonical chain, manages reorgs, and publishes `block.validated` and `chain.tip`. Separates hot header index (NVMe) from cold bodies (object storage). ([BSV Skills Center][7])

8. Block Storage Tiering — **Archivist**
   Implements hot/cold strategy: last 30–90 days on NVMe + all headers; older bodies in S3/MinIO with local index and prefetch window. (Cheaper archival nodes; fast recent queries.) ([BSV Skills Center][7])

9. Public API Gateway — **Portal**
   Unified front door for JSON-RPC, ARC-style endpoints, explorer queries, and BFFs (mobile SPV vs. enterprise). AuthN/Z, quotas, and request shaping. Routes to internal services or publishes events. (ARC is the living precedent for microservices TX processing and public endpoints.) ([Bitcoin SV][8], [TAAL Public Docs][9], [BSV Association][10])

10. Policy/Config — **Governor**
    Centralizes dynamic policies: standardness, fee/ancestor limits, relay thresholds, banlists, feature flags. Distributes signed snapshots to services; audits changes.

11. Observability — **Telemetry**
    Uniform logs/metrics/traces, topic lag dashboards, synthetic TX/Block canaries, SLO burn-rate alerts.

12. Bus/Foundation — **Nervous System**
    Kafka/Redpanda or NATS JetStream as the durable event log (partitioned topics, consumer groups). Backpressure is intrinsic; replay for recovery and audits. (This is the backbone that replaces a monolithic mempool.) ([BSV Blockchain][1])

---

## Event topics & synchronous APIs (contracts you can implement now)

**Topics (append-only, versioned):**

* `p2p.raw_tx.v1` → `{"txBytes":"base64","peerId":"str","seenAt":"rfc3339","net":"main|test|stn"}`
* `tx.validated.v1` → `{"txid":"hex","size":1234,"sigChecks":42,"validatedAt":"rfc3339"}`
* `tx.exec_request.v1` → `{"txid":"hex","vin":0,"scriptSig":"hex","scriptPubKey":"hex","amountSats":1234}`
* `tx.exec_result.v1` → `{"txid":"hex","vin":0,"ok":true,"error":null,"costUnits":321}`
* `tx.ready.v1` → `{"txid":"hex","feeRate":"sats/byte","ancestors":N,"weight":W}`
* `p2p.raw_block.v1`, `block.validated.v1`, `chain.tip.v1` (similar shapes)

**Ledger gRPC (sync for hot path):**

* `GetOutputs(Outpoints[]) -> Outputs[]` (idempotent; p99 ≤ 1 ms for N≤10)
* `ApplyBlock(BlockDiff) -> Ack{height, tipHash}` (atomic; watermark for consumers)

**Contract rules:** Protobuf with reserved fields; at-least-once delivery + idempotent consumers; partition keys by `txid`/`blockHash` for ordering within entity; topic suffix `.vN` for schema evolution.

---

## SLOs & scale targets (tie autoscaling to these)

* **Portal → tx.ready**: p99 ≤ 300 ms (single ancestor), ≤ 1.5 s (package ≤ 24 ancestors).
* **Verifier**: p99 ≤ 25 ms at 5k TPS/pod; scale on CPU + topic lag.
* **Ledger.GetOutputs**: cluster ≥ 5 M QPS, p99 ≤ 1 ms (10-input batch).
* **Chronicler block validate**: p99 ≤ 2 s for 256 MB block on hot data; reorg replay bounded by watermark windows.
  These are consistent with the direction of BSV’s microservices move (Teranode’s million-TPS testing & ARC’s scalable TX handling). ([BSV Blockchain][4], [Aerospike][3])

---

## Data stores & formats

* **UTXO**: Aerospike (or ScyllaDB) primary; Redis Cluster allowed for dev/proto. Teranode + Aerospike test results justify the choice for million-TPS scale. ([Aerospike][3], [BSV Blockchain][4])
* **Unconfirmed graph (Conductor)**: Badger/Rocks-like KV or FoundationDB; keys by `txid`, sets for ancestor/descendant.
* **Headers & indices (Chronicler)**: NVMe + columnar indices for height/hash/time; cold bodies in S3/MinIO. ([BSV Skills Center][7])
* **Event bus**: Redpanda or JetStream; replication factor ≥ 3; compaction on idempotent topics.

---

## Security & blast-radius containment

* **Sentinel**: per-peer quotas, token buckets, early cut-through checksum, strict payload caps; never executes consensus code.
* **Engine**: rlimits per script, op-count budgets, jailer/WASM isolation; kill-switch on cost units.
* **Bulkheads**: separate consumer groups/quotas per topic; circuit breakers on Ledger/Chronicler clients.
* **Admission control** at Portal (ARC-style API keys, per-tenant rate plans). ([Bitcoin SV][8], [TAAL Public Docs][9])

---

## Deployment blueprint (Kubernetes first)

* **Per-AZ cell**: Bus (3–5 nodes), Ledger (3+ stateful), Chronicler (3), Conductor/Verifier/Engine (autoscaled pools), Portals & Sentinels as edge sets.
* **Policies**: PodDisruptionBudgets; PriorityClasses (Ledger > Chronicler > Engine > Verifier > Conductor > Portal > Sentinel); topologySpreadConstraints; KEDA autoscaling on CPU + topic lag.
* **Traffic**: NodePort/LoadBalancer for Portal/Sentinel; mTLS for east-west; service discovery via k8s DNS.
  (Aligned with Teranode’s horizontally scalable, microservices direction.) ([BSV Blockchain][11])

---

## Phase plan (compatibility & risk control)

1. **Observer**: Full stack runs read-only off mainnet gossip; require ≥99.99% tx/block agreement vs. SV Node, <1-block lag. ([BSV Skills Center][12])
2. **Contributor**: Expose read APIs; relay only your own submitted tx; benchmark ARC-style flows with partners. ([Bitcoin SV][8], [BSV Association][10])
3. **Miner**: Produce/submit blocks; dual-run with SV Node until orphan deltas are zero. ([BSV Blockchain][1])
4. **Successor**: Decommission legacy paths after long-haul stability.

---

## Notes on terminology/claims (to keep you defensible)

* Prefer “full original Script opcodes restored; complex contracts via higher-layer patterns” instead of calling Script “Turing complete.”
* Anchor claims about microservices scalability and ARC/mAPI lineage to BSV Association/ARC docs and the Aerospike/100B-per-day reports. ([BSV Blockchain][1], [Bitcoin SV][8], [Aerospike][3])

---

# Project Metamorph — Service Contracts & K8s Reference (v0.1)

A compact, implementation-ready blueprint for a Teranode-class, microservices-based BSV node. This document defines event topics, synchronous APIs (gRPC), ordering/idempotency rules, and a Kubernetes reference topology with autoscaling hooks. It’s designed so teams can split into workstreams and ship immediately.

---

## 0) Scope & Non-Goals

**Scope:** Hot-path service contracts for transaction/block processing, UTXO reads/writes, script execution, and chain persistence; observability/security hooks; k8s manifests for a baseline cell.

**Non-Goals:** Detailed P2P protocol implementation specifics, miner pool/stratum internals, or exhaustive DevSecOps runbooks.

---

## 1) Event Bus ("Nervous System")

**Transport:** Kafka/Redpanda or NATS JetStream (durable). Replication ≥ 3, acks=all. TLS on brokers. Retention: 72h (hot topics), compaction for idempotent streams.

**Topic naming:** `<domain>.<event>.<version>` (e.g., `p2p.raw_tx.v1`).

**Partition keys:**

* TX topics: `txid`
* Block topics: `blockHash`
* Chain-tip topics: single partition (total ordering)

**Delivery semantics:** At-least-once. Consumers are **idempotent** and de-duplicate via `msgId`.

**Message envelope (common):**

```json
{
  "msgId": "01J7Y27M9H5P8XF0P7B7JTNQ5X",  // ULID
  "emittedAt": "2025-09-05T12:34:56Z",
  "traceId": "87d0...",
  "net": "main|test|stn",
  "payload": { /* event-specific */ }
}
```

### 1.1 Topic Schemas (v1)

**p2p.raw\_tx.v1** (produced by *Sentinel*)

```json
{
  "txBytes": "base64",
  "peerId": "string",
  "seenAt": "rfc3339"
}
```

**tx.validated.v1** (produced by *Verifier*)

```json
{
  "txid": "hex64",
  "size": 1234,
  "sigChecks": 42,
  "validatedAt": "rfc3339"
}
```

**tx.exec\_request.v1** (produced by *Conductor*)

```json
{
  "txid": "hex64",
  "vin": 0,
  "scriptSig": "hex",
  "scriptPubKey": "hex",
  "amountSats": 1234,
  "flags": ["SCRIPT_ENABLE_SIGHASH_FORKID", "..." ]
}
```

**tx.exec\_result.v1** (produced by *Engine*)

```json
{
  "txid": "hex64",
  "vin": 0,
  "ok": true,
  "error": null,
  "costUnits": 321,
  "durationMs": 2
}
```

**tx.ready.v1** (produced by *Conductor*)

```json
{
  "txid": "hex64",
  "feeRate": 1.2,          // sats/byte (or weight unit)
  "ancestors": 5,
  "weight": 12345,
  "priority": "high|normal|low"
}
```

**p2p.raw\_block.v1** (produced by *Sentinel*)

```json
{
  "blockBytes": "base64",
  "peerId": "string",
  "seenAt": "rfc3339"
}
```

**block.validated.v1** (produced by *Chronicler*)

```json
{
  "blockHash": "hex64",
  "height": 123456,
  "txCount": 123_456,
  "validatedAt": "rfc3339"
}
```

**chain.tip.v1** (produced by *Chronicler*)

```json
{
  "tipHash": "hex64",
  "height": 123456
}
```

> **Schema evolution:** bump suffix (`.v2`) on breaking changes. Use Protobuf for on-the-wire payloads; JSON here is illustrative.

---

## 2) Protobuf Contracts (gRPC & Event Payloads)

> Use `buf` for lint/build/generation. Language targets: Go (I/O-bound services) and Rust (Verifier/Engine/Ledger).

### 2.1 `ledger.proto`

```proto
syntax = "proto3";
package metamorph.ledger.v1;
option go_package = "github.com/yourorg/metamorph/gen/ledger/v1;ledgerv1";

message Outpoint { bytes txid = 1; uint32 vout = 2; }
message Output {
  bool is_unspent = 1;        // false if spent or missing
  uint64 amount_sats = 2;     // 0 when is_unspent=false
  bytes script_pub_key = 3;   // empty when is_unspent=false
}

message GetOutputsRequest { repeated Outpoint outpoints = 1; }
message GetOutputsResponse {
  repeated Output outputs = 1;       // 1:1 positional with request
  bytes watermark_tip_hash = 2;      // consistency hint
  uint64 height = 3;
}

// BlockDiff describes atomic UTXO changes for a block.
message TxSpend { bytes txid = 1; uint32 vout = 2; }
message NewOutput { bytes txid = 1; uint32 vout = 2; uint64 amount_sats = 3; bytes script_pub_key = 4; }
message ApplyBlockRequest {
  bytes block_hash = 1;
  uint64 height = 2;
  repeated TxSpend spends = 3;       // remove
  repeated NewOutput outputs = 4;    // add
  bytes prev_tip_hash = 5;           // optional precondition
}
message ApplyBlockResponse { bytes new_tip_hash = 1; uint64 height = 2; }

message HealthRequest {}
message HealthResponse { string status = 1; }

service Ledger {
  rpc GetOutputs(GetOutputsRequest) returns (GetOutputsResponse);
  rpc ApplyBlock(ApplyBlockRequest) returns (ApplyBlockResponse);
  rpc Health(HealthRequest) returns (HealthResponse);
}
```

**SLOs (non-binding, for autoscaling):**

* `GetOutputs`: p99 ≤ 1 ms (batch ≤ 10 inputs), cluster ≥ 5M QPS
* `ApplyBlock`: p99 ≤ 250 ms for 256 MB block diff on hot storage

### 2.2 `engine.proto`

```proto
syntax = "proto3";
package metamorph.engine.v1;
option go_package = "github.com/yourorg/metamorph/gen/engine/v1;enginev1";

message ExecuteRequest {
  bytes txid = 1;
  uint32 vin = 2;
  bytes script_sig = 3;
  bytes script_pub_key = 4;
  uint64 amount_sats = 5;
  uint32 flags = 6;            // script verification flags bitmask
  uint32 cpu_budget_us = 7;    // sandbox limits
  uint32 opcount_budget = 8;
}
message ExecuteResponse {
  bool ok = 1;
  string error = 2;            // empty on success
  uint32 cost_units = 3;       // engine-specific metering
  uint32 duration_ms = 4;
}

service Engine { rpc Execute(ExecuteRequest) returns (ExecuteResponse); }
```

### 2.3 `events.proto` (shared payloads for bus)

```proto
syntax = "proto3";
package metamorph.events.v1;

message Envelope {
  string msg_id = 1;     // ULID
  string emitted_at = 2; // RFC3339
  string trace_id = 3;
  string net = 4;        // main|test|stn
  bytes payload = 10;    // packed Any or a concrete message
}

message RawTx { bytes tx = 1; string peer_id = 2; string seen_at = 3; }
message TxValidated { bytes txid = 1; uint32 size = 2; uint32 sig_checks = 3; string validated_at = 4; }
message ExecReq { bytes txid = 1; uint32 vin = 2; bytes script_sig = 3; bytes script_pub_key = 4; uint64 amount_sats = 5; uint32 flags = 6; }
message ExecRes { bytes txid = 1; uint32 vin = 2; bool ok = 3; string error = 4; uint32 cost_units = 5; uint32 duration_ms = 6; }
message TxReady { bytes txid = 1; double fee_rate = 2; uint32 ancestors = 3; uint32 weight = 4; string priority = 5; }
message RawBlock { bytes block = 1; string peer_id = 2; string seen_at = 3; }
message BlockValidated { bytes block_hash = 1; uint64 height = 2; uint32 tx_count = 3; string validated_at = 4; }
message ChainTip { bytes tip_hash = 1; uint64 height = 2; }
```

---

## 3) Idempotency, Ordering, and Backpressure

* **Idempotency:** All events carry a `msgId` (ULID). Consumers maintain a last-N bloom/SET (or broker transactional markers) per topic+partition and discard duplicates.
* **Ordering:** Partition by `txid`/`blockHash`. Do **not** assume cross-partition order.
* **Backpressure:** Scale consumer replicas on lag (KEDA) + CPU. Producers never block; topics have bounded retention and DLQs for poison messages.

---

## 4) Kubernetes Reference Topology (single AZ cell)

**Namespaces**

* `metamorph-system`: bus, controllers, cert-manager, KEDA, OTel collector
* `metamorph-data`: Ledger, Chronicler, Archivist
* `metamorph-app`: Sentinel, Verifier, Engine, Conductor, Architect, Portal, Governor, Telemetry

**Pod priorities (highest → lowest):** Ledger > Chronicler > Engine > Verifier > Conductor > Portal > Sentinel > Telemetry

### 4.1 Boilerplate (Namespaces & RBAC)

```yaml
apiVersion: v1
kind: Namespace
metadata: { name: metamorph-system }
---
apiVersion: v1
kind: Namespace
metadata: { name: metamorph-data }
---
apiVersion: v1
kind: Namespace
metadata: { name: metamorph-app }
```

### 4.2 Verifier Deployment (stateless, autoscaled)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: verifier
  namespace: metamorph-app
spec:
  replicas: 3
  selector: { matchLabels: { app: verifier } }
  template:
    metadata:
      labels: { app: verifier }
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
    spec:
      containers:
      - name: verifier
        image: yourrepo/verifier:0.1.0
        ports: [{ containerPort: 9090 }]
        env:
        - { name: BUS_BROKERS, value: "redpanda-0:9092,redpanda-1:9092,redpanda-2:9092" }
        - { name: TOPIC_RAW_TX, value: "p2p.raw_tx.v1" }
        - { name: TOPIC_TX_VALIDATED, value: "tx.validated.v1" }
        - { name: CONSUMER_GROUP, value: "verifier-v1" }
        resources:
          requests: { cpu: "2", memory: "2Gi" }
          limits:   { cpu: "8", memory: "8Gi" }
        readinessProbe: { httpGet: { path: /healthz, port: 9090 }, periodSeconds: 5 }
        livenessProbe:  { httpGet: { path: /livez,  port: 9090 }, periodSeconds: 10 }
```

### 4.3 Ledger StatefulSet (backed by Aerospike/ScyllaDB client)

> In production, run the DB itself as a separate cluster (managed or bare-metal). This pod runs the **Ledger** service that *uses* that DB.

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ledger
  namespace: metamorph-data
spec:
  serviceName: ledger
  replicas: 3
  selector: { matchLabels: { app: ledger } }
  template:
    metadata:
      labels: { app: ledger }
    spec:
      containers:
      - name: ledger
        image: yourrepo/ledger:0.1.0
        ports: [{ containerPort: 8443 }]
        env:
        - { name: AEROSPIKE_HOSTS, value: "aero-0:3000,aero-1:3000,aero-2:3000" }
        - { name: TLS_ENABLED, value: "true" }
        volumeMounts:
        - { name: data, mountPath: /var/lib/ledger }
  volumeClaimTemplates:
  - metadata: { name: data }
    spec:
      accessModes: ["ReadWriteOnce"]
      resources: { requests: { storage: 200Gi } }
```

### 4.4 HPA (CPU) + KEDA ScaledObject (Kafka lag)

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: verifier-hpa
  namespace: metamorph-app
spec:
  scaleTargetRef: { apiVersion: apps/v1, kind: Deployment, name: verifier }
  minReplicas: 3
  maxReplicas: 200
  metrics:
  - type: Resource
    resource: { name: cpu, target: { type: Utilization, averageUtilization: 70 } }
---
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: verifier-keda
  namespace: metamorph-app
spec:
  scaleTargetRef: { name: verifier }
  pollingInterval: 5
  cooldownPeriod: 60
  minReplicaCount: 3
  maxReplicaCount: 500
  triggers:
  - type: kafka
    metadata:
      bootstrapServers: redpanda-0:9092,redpanda-1:9092,redpanda-2:9092
      topic: p2p.raw_tx.v1
      consumerGroup: verifier-v1
      lagThreshold: "10000"
```

### 4.5 Service Mesh & mTLS (optional but recommended)

* Install Linkerd/Istio. Enforce mTLS between services. Deny-all default NetworkPolicies; allow egress only to brokers/DBs.

---

## 5) Observability & Telemetry

* **Metrics:** Prometheus scraping; RED metrics per service; Kafka topic lag dashboards.
* **Tracing:** OpenTelemetry SDK → OTel Collector → Tempo/Jaeger.
* **Logs:** JSON to stdout → Loki/ELK. Include `traceId`, `msgId`, `txid`.
* **Canaries:** Synthetic `RawTx`/`RawBlock` injectors to validate E2E latency SLOs continuously.

---

## 6) Policy & Config (Governor)

* Signed policy snapshots (ed25519) distributed via ConfigMap or small gRPC API.
* Hot-reload in Verifier/Conductor/Portal for: standardness, fee floors, ancestor limits, relay thresholds, banlists.

---

## 7) Security & Blast Radius

* **Sentinel:** per-peer quotas, token buckets, payload caps; never executes consensus code.
* **Engine:** cgroups/rlimits; opcount/time budgets; kill-switch on cost units.
* **Bulkheads:** separate consumer groups + topic quotas; circuit breakers on Ledger/Chronicler clients.
* **Secrets:** all credentials from k8s Secrets; short-lived tokens where possible.

---

## 8) Makefile & buf.gen.yaml (snippet)

**Makefile**

```makefile
PROTO_DIR=proto
GEN_DIR=gen

.PHONY: proto
proto:
	buf generate $(PROTO_DIR)
```

**buf.gen.yaml**

```yaml
version: v1
plugins:
  - plugin: buf.build/protocolbuffers/go
    out: gen
    opt: paths=source_relative
  - plugin: buf.build/grpc/go
    out: gen
    opt: paths=source_relative
  - plugin: buf.build/protocolbuffers/python
    out: gen
```

---

## 9) Test Harness (Soak & Reorg)

* **Replay harness:** consume historical `p2p.raw_tx.v1`/`p2p.raw_block.v1` captures at 250k→1M TPS; verify `tx.ready` and `block.validated` parity with reference nodes.
* **Reorg tests:** randomized fork lengths (≤ 20 blocks) with timed `chain.tip.v1` updates; validate Ledger atomicity and Chronicler rollbacks.

---

## 10) Service Ownership Matrix

| Service    | Primary Lang | State | Scaling  | Owner Team |
| ---------- | ------------ | ----- | -------- | ---------- |
| Sentinel   | Go           | No    | HPA      | Networking |
| Verifier   | Rust         | No    | HPA/KEDA | Validation |
| Ledger     | Rust         | Yes   | Stateful | Data       |
| Engine     | Rust         | No    | HPA/KEDA | Scripting  |
| Conductor  | Go           | Yes   | HPA      | TX Flow    |
| Architect  | Go           | Yes   | HPA      | Mining     |
| Chronicler | Go           | Yes   | Stateful | Consensus  |
| Archivist  | Go           | Yes   | Stateful | Data       |
| Portal     | Go           | No    | HPA      | API        |
| Governor   | Go           | Yes   | HPA      | Platform   |
| Telemetry  | Go           | Yes   | HPA      | SRE        |

---

### Appendix A — Minimal Consumer Idempotency

```pseudo
onMessage(m):
  if seenCache.contains(m.msgId): return
  process(m)
  seenCache.add(m.msgId)
  if seenCache.size() > N: evictLRU()
```

### Appendix B — Partitioning & Keys

* Use canonical hex-lowercase `txid` (big-endian) for partitioning on all TX topics.
* Blocks use big-endian `blockHash`.

---

**End v0.1** — Propose freezing contracts and starting stub repos:

* `metamorph-proto/` (this doc + .proto files)
* `metamorph-verifier/` (Rust, consumes `p2p.raw_tx.v1` → produces `tx.validated.v1`)
* `metamorph-ledger/` (Rust, gRPC; uses Aerospike/Scylla client)
* `metamorph-engine/` (Rust, gRPC; sandboxed exec)
* `metamorph-conductor/` (Go; orchestrates; durable store)
* `metamorph-chronicler/` (Go; chain state; emits `block.validated`/`chain.tip`)
* `metamorph-portal/` (Go; API/BFF)


///

All set! I’ve dropped **“Metamorph Service Contracts (v0.1)”** into the canvas with event topics, protobuf gRPC stubs, and k8s reference manifests (HPA + KEDA). Want me to also split those into separate `.proto` and `.yaml` files so you can push them straight to repos?
