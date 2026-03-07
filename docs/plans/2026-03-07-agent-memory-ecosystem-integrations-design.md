# TiDB Agent Memory P1 Design: Ecosystem Integrations (mem0, LangGraph, MCP)

- Author(s): TBD
- Discussion PR: TBD
- Tracking Issue: TBD

## Overview

To make TiDB memory features usable in real agent stacks, we need stable integration contracts
for popular orchestration and memory frameworks. This design defines integration patterns,
SDK contracts, and reference adapters for mem0-style memory layers, LangGraph stores, and MCP tools.

## Goals

- Provide first-party integration contracts instead of ad hoc SQL snippets.
- Keep integrations protocol-driven and avoid vendor lock-in.
- Ensure idempotent writes and deterministic retrieval behavior across frameworks.
- Make migration and operations straightforward for existing agent stacks.

## Non-Goals

- Re-implement full framework runtimes inside TiDB.
- Depend on one specific embedding provider.

## Integration Scope

### Supported Surfaces

1. SQL-first contract (all features reachable from SQL).
2. gRPC/HTTP adapter service (optional sidecar, not TiDB server API change in P1).
3. Language SDK wrappers (Go/Python/TypeScript).

### Reference Targets

- mem0-like memory CRUD and retrieval workflows.
- LangGraph store/search/checkpoint workflows.
- MCP tool-call style retrieval and write actions.

## Detailed Design

### Canonical Operation Set

All adapters map to this operation set:

1. `UpsertMemory`
2. `RetrieveMemory`
3. `AssembleContext`
4. `ListMemory`
5. `DeleteMemory`
6. `ApplyLifecyclePolicy`
7. `GetTrace`

Each operation has strict idempotency requirements with client-supplied request id.

### Idempotency and Consistency

New table: `mysql.tidb_agent_memory_request_dedup`

- `request_id`, `tenant_id`, `op_type`, `status`, `result_ref`, `created_at`

Behavior:

- repeated request id returns prior result reference.
- upsert and delete operations remain transactional.

### mem0-style Adapter Profile

Mapping principles:

- memory extract/update phases map to `UpsertMemory` with provenance fields.
- memory search maps to hybrid retrieval operator.
- memory consolidation hooks map to lifecycle summarize/dedup jobs.

### LangGraph Adapter Profile

Mapping principles:

- BaseStore put/get/search map to canonical operation set.
- semantic search query maps to `RetrieveMemory` with vector+filter options.
- checkpoint metadata can be stored in a dedicated table keyed by thread id.

### MCP Tool Profile

Expose tool interfaces (via sidecar adapter) such as:

- `memory.retrieve`
- `memory.upsert`
- `memory.context.pack`
- `memory.trace.get`

The adapter enforces tenant context and policy checks before SQL execution.

### SDK Contracts

SDK minimum contracts:

- explicit tenant and namespace parameters
- typed request/response structs
- retry-safe idempotent methods
- error taxonomy aligned with SQL error codes

### Deployment Model

P1 recommends sidecar adapter deployment:

- keeps TiDB wire protocol unchanged.
- centralizes auth, rate limits, and payload validation.
- supports gradual rollout per workload.

### Compatibility and Versioning

- adapter capability negotiation endpoint returns supported profile version.
- server-side profile version lives in `mysql.tidb_agent_memory_profile_version`.
- backward-compatible additive fields only in minor versions.

## Test Design

### Functional Tests

- operation mapping correctness for each adapter profile.
- idempotency behavior under retries and network timeouts.
- tenant policy enforcement through adapter.

### Scenario Tests

- framework migration from app-side memory logic to TiDB adapter.
- mixed clients (SQL direct + adapter) in same dataset.

### Compatibility Tests

- profile-version negotiation with older SDK.
- behavior when optional capabilities are unavailable.

### Benchmark Tests

- adapter overhead vs direct SQL baseline.
- throughput under high-concurrency write and retrieve mix.

## Impacts and Risks

Impacts:

- much faster adoption in existing agent ecosystems.
- lower integration defects via consistent contracts.

Risks:

- adapter layer can become another operational component to maintain.
- capability drift if SQL and adapter versions diverge.

Mitigations:

- conformance test suite per profile.
- explicit compatibility matrix and semantic version policy.

## Investigation and Alternatives

- Alternative A: only publish SQL examples.
  - Rejected: high repeated integration cost and fragile behavior.
- Alternative B: tightly couple to one framework SDK.
  - Rejected: lock-in and limited ecosystem reach.

## Open Questions

- should adapter sidecar be maintained in TiDB repo or separate project?
- should MCP profile include streaming retrieval in P2?
