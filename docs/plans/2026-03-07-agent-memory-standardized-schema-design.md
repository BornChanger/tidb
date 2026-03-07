# TiDB Agent Memory P1 Design: Standardized Memory Schema

- Author(s): TBD
- Discussion PR: TBD
- Tracking Issue: TBD

## Overview

Teams repeatedly re-invent memory schemas and often miss provenance, quality, and governance
fields required in production. This design defines a standardized TiDB memory schema profile
covering episodic, semantic, and procedural memory, plus migration/versioning conventions.

## Goals

- Provide a canonical schema profile for agent memory workloads.
- Ensure interoperability across retrieval, lifecycle, and context assembly features.
- Standardize provenance and quality metadata for auditability.
- Support schema evolution without downtime.

## Non-Goals

- Force all existing memory datasets to migrate immediately.
- Enforce one fixed payload shape across all applications.

## Schema Profile

### Core Tables

1. `agent_memory_episodic`
   - event-like facts tied to interactions.
2. `agent_memory_semantic`
   - normalized, durable facts and concepts.
3. `agent_memory_procedural`
   - reusable procedures, playbooks, and actions.

Shared mandatory columns:

- `memory_id BIGINT PRIMARY KEY`
- `tenant_id VARCHAR(64)`
- `namespace VARCHAR(128)`
- `subject_id VARCHAR(128)`
- `payload JSON`
- `embedding VECTOR(D)`
- `importance DOUBLE`
- `confidence DOUBLE`
- `created_at TIMESTAMP`
- `updated_at TIMESTAMP`
- `expires_at TIMESTAMP NULL`
- `provenance JSON`
- `quality JSON`
- `state ENUM('hot','warm','cold','archived')`

### Index Recommendations

- vector index on `embedding`
- B-tree `(tenant_id, namespace, subject_id, created_at)`
- optional JSON path index for high-cardinality metadata fields

## Detailed Design

### Bootstrap SQL Package

P1 ships a bootstrap routine:

```sql
CALL mysql.tidb_agent_memory_bootstrap(
  schema_name => 'app_memory',
  embedding_dim => 1536,
  profile_version => 'v1'
);
```

The routine creates tables, indexes, comments, and compatibility views.

### Profile Versioning

New catalog table:

`mysql.tidb_agent_memory_profile_version`

- `schema_name`, `profile_version`, `applied_at`, `checksum`

Migration strategy:

- additive columns in minor profile versions.
- breaking changes require new compatibility views and migration scripts.

### Provenance and Quality Contracts

`provenance` JSON expected keys:

- `source_type` (chat/tool/doc/system)
- `source_id`
- `source_span_id` (optional tracing link)
- `ingest_pipeline_version`

`quality` JSON expected keys:

- `validator_version`
- `quality_score`
- `flags` (noise/duplication/risk tags)

### Compatibility Views

Standard views:

- `agent_memory_all`: `UNION ALL` across episodic/semantic/procedural with type label.
- `agent_memory_active`: excludes archived rows.
- `agent_memory_for_retrieval`: projects retrieval-required columns only.

These views let application code stay stable during underlying schema updates.

### Constraint Rules

- `confidence` and `importance` in `[0,1]` via CHECK constraints.
- non-null `tenant_id`, `namespace`, `payload`, `created_at`.
- unique tuple `(tenant_id, namespace, source_id, source_type)` optional for dedup profile.

### Integration with Other Features

- Hybrid retrieval expects `embedding`, `importance`, `created_at`, `metadata/payload`.
- Lifecycle engine uses `state`, `expires_at`, `quality`.
- Context assembly reads compatibility view `agent_memory_for_retrieval`.

### Upgrade and Downgrade

- bootstrap stores profile metadata so rollback scripts can detect current state.
- downgrade keeps compatibility views to avoid immediate app breakage.

## Test Design

### Functional Tests

- bootstrap creates all expected tables and indexes.
- constraints reject invalid confidence/importance values.
- compatibility views return consistent columns.

### Scenario Tests

- mixed-type retrieval through `agent_memory_all`.
- profile minor upgrade with zero app SQL changes.

### Compatibility Tests

- interaction with TiCDC and BR restore.
- migration under partitioned deployment.

### Benchmark Tests

- insert throughput across three table types.
- retrieval latency via compatibility views.

## Impacts and Risks

Impacts:

- lower onboarding cost for agent-memory projects.
- better interoperability for future TiDB-native memory features.

Risks:

- over-standardization can reduce flexibility for niche workloads.
- too many mandatory columns may increase write cost.

Mitigations:

- profile includes optional extension columns.
- compatibility views isolate most app SQL from schema details.

## Investigation and Alternatives

- Alternative A: publish examples only, no bootstrap contract.
  - Rejected: inconsistent adoption and drift.
- Alternative B: single monolithic memory table.
  - Rejected: weak separation of retention and quality rules by memory type.

## Open Questions

- Should profile include built-in partition strategy defaults by tenant?
- Should procedural memory have stricter schema than generic JSON payload?
