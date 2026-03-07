# TiDB Agent Memory P0 Design: Memory Lifecycle Engine

- Author(s): TBD
- Discussion PR: TBD
- Tracking Issue: TBD

## Overview

Agent memory storage grows quickly and degrades retrieval quality if old, low-value, or duplicate
entries are never consolidated. This design introduces a lifecycle engine that manages memory in
four states: `hot`, `warm`, `cold`, and `archived`, with policy-driven compaction and expiration.

## Goals

- Provide built-in lifecycle governance for memory tables.
- Keep memory quality high with summarization and dedup actions.
- Reuse existing TiDB TTL scheduling model where practical.
- Make lifecycle actions auditable and reversible when possible.

## Non-Goals

- Implement LLM summarization engine inside TiDB in P0.
- Define one universal memory policy for all applications.
- Replace BR/TiCDC backup and replication semantics.

## Design Principles

- Policy-driven: table-level policy controls transitions and retention.
- Asynchronous: lifecycle operations run in background workers.
- Explainable: each transition records reason and job id.
- Safe by default: do not delete rows without explicit policy.

## Detailed Design

### SQL Surface

P0 introduces lifecycle policy DDL:

```sql
ALTER TABLE agent_memories
  SET AGENT MEMORY POLICY (
    HOT_WINDOW = INTERVAL 7 DAY,
    WARM_WINDOW = INTERVAL 30 DAY,
    COLD_WINDOW = INTERVAL 180 DAY,
    ARCHIVE_AFTER = INTERVAL 365 DAY,
    SUMMARIZE_EVERY = INTERVAL 1 DAY,
    DEDUP_STRATEGY = 'semantic_threshold:0.92',
    ENABLE_DELETE = ON
  );
```

P0 also introduces admin controls:

```sql
ADMIN PAUSE AGENT MEMORY JOB <job_id>;
ADMIN RESUME AGENT MEMORY JOB <job_id>;
ADMIN CANCEL AGENT MEMORY JOB <job_id>;
```

### Metadata and System Tables

New system tables under `mysql`:

1. `mysql.tidb_agent_memory_policy`
   - `table_id`, `policy_json`, `enabled`, `updated_at`
2. `mysql.tidb_agent_memory_job`
   - `job_id`, `table_id`, `job_type`, `state`, `owner_id`, `start_time`, `finish_time`, `summary`
3. `mysql.tidb_agent_memory_transition_log`
   - `table_id`, `memory_id`, `from_state`, `to_state`, `reason`, `job_id`, `event_time`

### Lifecycle State Machine

```text
hot -> warm -> cold -> archived
```

Transition triggers:

- age threshold crossing
- dedup collapse
- summarize-and-replace action

Rules:

- rows in `archived` are not considered by retrieval operators by default.
- if `ENABLE_DELETE=ON`, archived rows are deleted after retention period.
- if `ENABLE_DELETE=OFF`, archived rows remain queryable with explicit opt-in.

### Background Job Model

Reuse TTL-style scheduling model:

- scheduler periodically scans tables with enabled memory policy.
- one active lifecycle job per physical table.
- each job emits scan tasks and action tasks.

Action task types:

1. `transition`: update state and metadata.
2. `summarize`: write summary row (external summarizer callback or SQL UDF).
3. `dedup`: mark duplicate groups and keep canonical row.
4. `expire`: hard delete when policy allows.

### Summarization Contract

P0 does not require in-database model inference. Instead, it defines a callback contract:

- input: candidate row set + schema + policy context.
- output: `summary_payload`, `source_memory_ids`, `quality_score`.

The output is inserted as a new memory row with provenance metadata.

### Failure Handling

- idempotent task execution via `(job_id, task_id)` key.
- checkpoint progress in job state.
- on owner failure, another node can resume from last checkpoint.
- partial failures are reported in `summary` and metrics.

### Compatibility

- parser: new table option extension.
- DDL: policy metadata update should be online.
- planner/executor: retrieval operator filters out archived by default.
- BR/TiCDC: system tables included; behavior documented for restore.
- upgrade/downgrade: gated by `tidb_enable_agent_memory_lifecycle`.

## Test Design

### Functional Tests

- DDL set/update/remove lifecycle policy.
- state transitions by synthetic timestamps.
- pause/resume/cancel behavior.
- delete guard when `ENABLE_DELETE=OFF`.

### Scenario Tests

- failover during summarize task.
- large tenant with mixed hot/cold ratios.
- dedup false-positive tolerance checks.

### Compatibility Tests

- partitioned tables with per-partition jobs.
- interaction with existing TTL tables.

### Benchmark Tests

- scheduler overhead with 1k managed tables.
- transition throughput under write-heavy workload.

## Impacts and Risks

Impacts:

- lowers memory storage cost and stale-context noise.
- improves retrieval precision over long-lived agents.

Risks:

- aggressive policy can delete useful long-tail memory.
- summarization quality drift can lose details.

Mitigations:

- safe defaults (`ENABLE_DELETE=OFF` initially).
- provenance links from summary rows to source rows.
- policy dry-run mode in P1.

## Investigation and Alternatives

- Alternative A: require applications to own lifecycle fully.
  - Rejected: weak consistency and no shared observability.
- Alternative B: delete-only policy without warm/cold transitions.
  - Rejected: cannot support quality-preserving consolidation.

## Open Questions

- Should lifecycle policies support per-namespace overrides in P1?
- Should dedup strategy include cross-table candidates?
