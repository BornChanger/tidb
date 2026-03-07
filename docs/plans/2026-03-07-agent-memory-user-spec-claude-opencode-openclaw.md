# TiDB Agent Memory User Spec (Claude Code / OpenCode / OpenClaw)

- Author(s): TBD
- Discussion PR: TBD
- Tracking Issue: TBD

## 1. Purpose

This specification defines the user-facing behavior for integrating TiDB Agent Memory with:

1. Claude Code
2. OpenCode
3. OpenClaw

The goal is to provide a consistent memory experience across all three clients while preserving
TiDB-native guarantees for isolation, durability, observability, and compatibility.

## 2. Target Users and Primary Scenarios

### Personas

- Individual developer using one coding agent.
- Team using multiple agents sharing the same workspace memory.
- Platform operator managing multi-tenant memory infrastructure.

### Primary Scenarios

1. Persistent memory across sessions and restarts.
2. Team-shared memory with tenant/space isolation.
3. Hybrid semantic + keyword recall under latency budget.
4. Controlled lifecycle for stale or low-value memory.
5. Explainable context assembly for debugging model behavior.

## 3. Product Surface

### 3.1 Server Surface (Required)

TiDB Agent Memory exposes a server API as the stable client contract.

Core operation set:

- `memory.store`
- `memory.search`
- `memory.get`
- `memory.update`
- `memory.delete`
- `memory.context.pack`
- `memory.trace.get`

### 3.2 SQL Surface (Required)

Server APIs map to TiDB-native SQL/operator capabilities for:

- hybrid retrieval (`HR`)
- lifecycle policy and transitions (`LC`)
- context assembly (`CA`)
- trace/explain (`OX`)

## 4. Client Integration Requirements

### 4.1 Claude Code Integration

Required behavior:

1. On session start, fetch recent/relevant memories and inject concise context.
2. On stop/session end, persist summarized assistant output when policy allows.
3. Expose on-demand operations for explicit save and recall.

Integration hooks:

- `SessionStart` -> pre-load memory snippets
- `UserPromptSubmit` -> memory hinting/availability
- `Stop` -> capture and persist terminal summary

User controls:

- disable auto-capture
- set memory namespace
- set maximum recalled memory count

### 4.2 OpenCode Integration

Required behavior:

1. On prompt build, inject relevant memories into system context.
2. Expose tool-level CRUD + search operations.
3. Use idempotent write semantics for retries.

Integration hooks/tools:

- `system.transform` hook for recall injection
- `memory_store`, `memory_search`, `memory_get`, `memory_update`, `memory_delete`

User controls:

- toggle automatic memory injection
- configure retrieval depth and token budget
- choose namespace and team space

### 4.3 OpenClaw Integration

Required behavior:

1. Replace built-in memory slot with TiDB-backed memory plugin.
2. Auto-recall before prompt build with bounded cache TTL.
3. Auto-capture before reset and at agent end with policy checks.

Integration hooks/tools:

- hooks: `before_prompt_build`, `after_compaction`, `before_reset`, `agent_end`
- tools: `memory_store`, `memory_search`, `memory_get`, `memory_update`, `memory_delete`

User controls:

- configure tenant/space binding
- configure cache TTL and auto-capture thresholds
- disable specific lifecycle hooks per deployment policy

## 5. Tenant and Identity Model

### Required Fields

- `tenant_id`
- `namespace`
- `subject_id` (user/agent/session principal)
- `request_id` (for idempotency)

### Rules

1. Every client request must include tenant context.
2. Missing tenant context must fail closed.
3. Cross-tenant data access is disallowed by default.

## 6. Data and Retrieval Behavior

### 6.1 Memory Types

- `episodic`
- `semantic`
- `procedural`

### 6.2 Retrieval Contract

Search should support:

- hybrid ranking (`vector + recency + importance + structured filters`)
- deterministic top-k ordering under fixed inputs
- bounded candidate expansion to prevent cost spikes

### 6.3 Context Assembly Contract

`memory.context.pack` should return:

- assembled context text
- selected memory ids and score metadata
- estimated token usage
- dropped reasons for excluded candidates

## 7. Lifecycle and Retention

Policy states:

- `hot`, `warm`, `cold`, `archived`

Required behavior:

1. Archived memories are excluded from default recall.
2. Deletion must be explicit and auditable.
3. Lifecycle transitions must be idempotent and recoverable.

## 8. Observability and Explainability

Required outputs:

- per-request trace id
- retrieval/assembly latency metrics
- candidate/selected/dropped counts
- explainable score decomposition for selected memories

Privacy defaults:

- traces and logs should avoid raw sensitive payload unless explicitly enabled.

## 9. Non-Functional Requirements

### Performance (initial targets)

- Recall request P99 <= 150ms for typical top-k queries.
- Context pack generation P99 <= 220ms for bounded candidate set.

### Reliability

- idempotent write semantics under retry.
- no silent data loss on plugin restart.

### Compatibility

- additive evolution for client-facing API fields.
- explicit version negotiation for adapter profiles.

## 10. Security Requirements

1. Tenant isolation across all entry points (SQL and adapter APIs).
2. Redaction controls for classified payloads.
3. Auditable operations for read/write/delete and policy denials.
4. Optional payload-capture controls for debugging with explicit opt-in.

## 11. User Experience Requirements

### Minimum UX Guarantees

- Users can understand when memory was recalled and why.
- Users can explicitly store, search, and forget memory.
- Users can disable auto-capture and adjust memory scope.

### Error UX

- authentication/tenant errors return actionable messages.
- budget/policy errors explain what setting caused rejection.

## 12. Acceptance Criteria

This spec is satisfied when all are true:

1. Claude Code/OpenCode/OpenClaw integrations support the required operation set.
2. Tenant isolation and fail-closed behavior are validated end-to-end.
3. Hybrid recall and context assembly behavior match contracts.
4. Trace/explain outputs are available and privacy-safe by default.
5. Lifecycle behavior and deletion workflow are auditable.

## 13. Out of Scope (This Spec)

- Provider-specific prompt engineering behavior.
- In-database LLM inference implementation details.
- UI/dashboard product design for operators.

## 14. References

- `docs/plans/2026-03-07-agent-memory-design-overview-index.md`
- `docs/plans/2026-03-07-agent-memory-milestone-a-detailed-design.md`
- `docs/plans/2026-03-07-agent-memory-milestone-b-detailed-design.md`
- `docs/plans/2026-03-07-agent-memory-milestone-c-detailed-design.md`
- `docs/plans/2026-03-07-agent-memory-ecosystem-integrations-design.md`
