# TiDB Agent Memory P0 Design: Hybrid Retrieval Operator

- Author(s): TBD
- Discussion PR: TBD
- Tracking Issue: TBD

## Overview

This document proposes a TiDB-native hybrid retrieval operator for agent memory workloads.
The operator combines vector similarity, structured filters, recency decay, and business
importance into one optimizer-aware query pattern, so applications can avoid ad hoc ranking
logic outside SQL.

## Goals

- Provide deterministic and explainable hybrid ranking for agent memory retrieval.
- Keep retrieval in SQL so applications can use transactions, ACL, and observability already in TiDB.
- Preserve compatibility with current vector-index capability and regular SQL filters.
- Support low-latency top-k retrieval with bounded CPU and predictable performance.

## Non-Goals

- Replace all application-level ranking logic in one release.
- Introduce a full-text engine in P0.
- Change TiKV storage format for normal rows.

## Workload and Requirements

Target query pattern:

1. Filter by tenant/user/session/namespace.
2. Retrieve top-N by semantic similarity to the current query embedding.
3. Re-rank with recency and importance.
4. Return top-K entries for downstream prompt assembly.

Functional requirements:

- R1: rank quality should be reproducible for the same inputs and weights.
- R2: support per-query weight overrides.
- R3: support hard filter predicates before final ranking.

Performance requirements:

- P99 latency <= 150 ms for K <= 50 on warmed cache, medium dataset.
- Re-ranking candidate pool should be bounded by a configurable upper limit.

## Detailed Design

### SQL Surface

P0 adds a table function:

```sql
SELECT *
FROM TIDB_AGENT_MEMORY_RETRIEVE(
  tenant_id       => 't-1',
  namespace       => 'assistant-default',
  query_vector    => '[0.12, -0.05, ...]',
  limit_k         => 20,
  candidate_n     => 200,
  recency_half_life_seconds => 604800,
  weight_vector   => 0.55,
  weight_recency  => 0.25,
  weight_importance => 0.20,
  max_age_seconds => 7776000,
  metadata_filter => '{"topic":"billing"}'
);
```

Return columns:

- `memory_id`, `tenant_id`, `namespace`, `payload`, `metadata`, `created_at`
- `vector_score`, `recency_score`, `importance_score`, `final_score`
- `rank_reason` (compact JSON describing score decomposition)

### Scoring Model

For each candidate row:

```text
vector_score     = 1 - normalized_distance
recency_score    = exp(-age_seconds / half_life_seconds)
importance_score = clamp(importance, 0, 1)
final_score      = wv*vector_score + wr*recency_score + wi*importance_score
```

Rules:

- weights default to `(0.55, 0.25, 0.20)` and must sum to 1.0 (with tolerance 1e-6).
- if `importance` is NULL, use `0.5`.
- if `candidate_n < limit_k`, fallback to `candidate_n = limit_k`.

### Optimizer and Planner

New logical operator: `LogicalAgentMemoryRetrieve`.

- During logical optimization, convert function call into:
  1. vector top-N candidate scan (index-backed)
  2. metadata predicate filter
  3. re-ranking projection
  4. top-K projection
- push down tenant/namespace/time-range predicates before re-ranking.
- estimate cost from `candidate_n`, vector distance compute cost, and row width.

New physical operator: `PhysicalAgentMemoryRetrieve`.

- Preferred path: use vector index access + local re-ranking.
- Fallback path: table scan with warning when vector index is absent.

### Execution Flow

1. Validate arguments and defaulting.
2. Build candidate set from vector index.
3. Apply hard filters (`tenant`, `namespace`, metadata, age).
4. Compute score components in vectorized executor path.
5. Top-K heap returns final result.
6. Attach `rank_reason` for explainability.

### Storage and Indexing Requirements

Recommended memory table columns:

- `embedding VECTOR(D)`
- `tenant_id`, `namespace`, `created_at`, `importance`
- `metadata JSON`, `payload JSON`

Recommended indexes:

- vector index on `embedding`
- composite B-tree index on `(tenant_id, namespace, created_at)`

### Error Handling

- Invalid weight sum: return SQL error `ErrAgentMemoryInvalidWeights`.
- Invalid vector dim: return SQL error `ErrAgentMemoryVectorDimMismatch`.
- Missing vector index: continue with degraded path + warning.

### Compatibility

- Parser: no syntax extension required in P0 (table function only).
- Planner/Executor: new operator added behind feature flag.
- TiFlash: optional acceleration, not required.
- BR/TiCDC: no special handling; data remains regular table rows.
- Upgrade/downgrade: feature-gated by system variable.

## Test Design

### Functional Tests

- ranking determinism for fixed input and fixed weights.
- weight override behavior.
- tenant isolation and metadata filter correctness.
- warning behavior when vector index is missing.

### Scenario Tests

- cold-start retrieval with sparse history.
- high-cardinality namespace retrieval.
- recency-sensitive recall for same semantic distance.

### Compatibility Tests

- coexistence with partition tables and generated columns.
- privilege checks under restricted users.

### Benchmark Tests

- latency vs candidate pool (`candidate_n = 50/100/200/500`).
- throughput under mixed read/write memory workloads.

## Impacts and Risks

Impacts:

- significant reduction of app-side ranking code.
- better explainability and stable retrieval behavior.

Risks:

- poor defaults can bias recency too much and reduce semantic quality.
- large candidate pools can create CPU spikes.

Mitigations:

- conservative defaults + explicit guardrails.
- per-tenant limits on `candidate_n` and query rate.

## Investigation and Alternatives

- Alternative A: perform all fusion in application layer.
  - Rejected: poor consistency and weak explainability.
- Alternative B: only vector top-k without fusion.
  - Rejected: weak personalization and stale result quality.

## Open Questions

- Should lexical score become first-class in P1 if TiDB adds full-text primitive?
- Should `rank_reason` be JSON text or structured columns for lower overhead?
