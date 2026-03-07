# TiDB Agent Memory P1 Design: Observability and Explainability

- Author(s): TBD
- Discussion PR: TBD
- Tracking Issue: TBD

## Overview

Production agent systems fail in ways that are hard to diagnose without detailed retrieval and
context traces. This design introduces TiDB-native observability and explainability primitives
for agent-memory workloads, including score decomposition, assembly traces, and standard telemetry.

## Goals

- Make retrieval and context assembly decisions inspectable.
- Expose low-overhead metrics for latency, quality proxies, and cost.
- Align telemetry fields with OpenTelemetry GenAI conventions where applicable.
- Support root-cause analysis for bad answers and noisy recalls.

## Non-Goals

- Store full prompts/responses by default in TiDB system tables.
- Build a full APM product inside TiDB.

## Detailed Design

### Explain Surfaces

P1 adds two explain surfaces:

1. `EXPLAIN AGENT MEMORY <query>`
   - plan-level view of candidate generation and ranking pipeline.
2. `SHOW AGENT MEMORY TRACE <trace_id>`
   - run-level trace summary from system table logs.

### Trace Data Model

System table: `mysql.tidb_agent_memory_trace`

- `trace_id`, `stmt_digest`, `tenant_id`, `namespace`
- `query_hash`, `candidate_n`, `selected_k`
- `vector_ms`, `filter_ms`, `rerank_ms`, `assembly_ms`, `total_ms`
- `token_budget`, `input_tokens_estimated`, `dropped_count`
- `error_code`, `warning_flags`
- `created_at`

System table: `mysql.tidb_agent_memory_trace_item`

- `trace_id`, `memory_id`, `rank`, `vector_score`, `recency_score`, `importance_score`, `final_score`
- `selected BOOL`, `dropped_reason`

Retention defaults to 7 days with configurable TTL.

### Metrics

Prometheus metrics:

- `tidb_agent_memory_retrieve_latency_ms`
- `tidb_agent_memory_assemble_latency_ms`
- `tidb_agent_memory_candidate_count`
- `tidb_agent_memory_selected_count`
- `tidb_agent_memory_drop_ratio`
- `tidb_agent_memory_budget_utilization`
- `tidb_agent_memory_error_total`

Dimensions include `tenant`, `namespace`, `operation`, `status` with cardinality safeguards.

### OpenTelemetry Mapping

When tracing is enabled, TiDB attaches optional span attributes aligned with GenAI semconv style:

- operation name (`retrieve_memory`, `assemble_context`)
- model/provider tags if provided by client side session vars
- token estimate and budget fields
- error type and warning codes

Large prompt payloads are not emitted by default. Only references/hashes are emitted unless explicit opt-in.

### Slow Query and Statement Summary Integration

Additional fields in slow log and statement summary extension:

- `AgentMemoryCandidateN`
- `AgentMemorySelectedK`
- `AgentMemoryAssembleTokens`
- `AgentMemoryDropRatio`

These fields are emitted only when agent-memory operators are used.

### Sampling and Cost Control

System variables:

- `tidb_agent_memory_trace_enable`
- `tidb_agent_memory_trace_sample_ratio`
- `tidb_agent_memory_trace_max_items`

Sampling strategy:

- always sample error traces
- probabilistic sample for successful requests

### Security and Privacy Considerations

- trace tables store score components and ids, not raw sensitive payload by default.
- explicit opt-in required for extended payload capture.

### Compatibility

- parser: new explain/show statements are additive.
- planner/executor: no behavior change when tracing disabled.
- TiCDC/BR: trace tables can be excluded by policy in backup configs.

## Test Design

### Functional Tests

- explain output contains hybrid retrieval pipeline nodes.
- trace tables populated under sampled requests.
- score decomposition matches execution path values.

### Scenario Tests

- high-QPS retrieval with low sample ratio.
- forced error path captures required diagnostics.

### Compatibility Tests

- no trace leakage under restricted privileges.
- co-existence with normal SQL observability flows.

### Benchmark Tests

- overhead of tracing enabled vs disabled.
- storage footprint under different sample ratios.

## Impacts and Risks

Impacts:

- faster troubleshooting and safer tuning cycles.
- improved confidence for enterprise rollout.

Risks:

- trace storage growth if sampling is misconfigured.
- high-cardinality tags can increase telemetry cost.

Mitigations:

- hard cap for trace item count per statement.
- explicit cardinality guardrails and tag allow-list.

## Investigation and Alternatives

- Alternative A: rely only on external APM.
  - Rejected: loses TiDB-internal ranking and assembly details.
- Alternative B: no per-item traces, only aggregates.
  - Rejected: weak explainability for incorrect recalls.

## Open Questions

- should trace tables support cross-cluster export hooks?
- should we provide a normalized JSON schema for rank reasoning interoperability?
