# TiDB Agent Memory Milestone Issue Backlog Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Provide a directly actionable GitHub issue backlog (epics + sub-issues + acceptance criteria) aligned to Milestone A/B/C detailed designs.

**Architecture:** The backlog is organized by milestone contracts, with explicit hard dependencies and acceptance gates. Each issue is designed to be independently reviewable while preserving cross-track integration order.

**Tech Stack:** TiDB SQL, planner/executor/session layers, system tables, docs/plans contracts, integration tests, benchmark validations.

---

## Usage

Create issues in the order below. For each issue, use:

- **Title** exactly as specified.
- **Suggested labels** from this document.
- **Body template** from the issue block.

All issue titles and descriptions are in English to match repository issue guidance.

## Execution Rules (Mandatory)

### Dependency Semantics

- `Hard`: downstream issue cannot start implementation before dependency is merged.
- `Soft`: downstream issue can start design/prototype work but cannot be closed before dependency is merged.

### Issue Close Rule

An issue can be closed only when all conditions are true:

1. all hard dependencies are merged,
2. all acceptance criteria in the issue body are checked,
3. validation evidence (exact commands + outputs summary) is attached,
4. compatibility block is filled (upgrade/downgrade/mixed-version expectation).

### Required Body Blocks for Every Created Issue

Each GitHub issue created from this backlog should contain these sections:

1. `## Summary`
2. `## Scope`
3. `## Acceptance Criteria` (binary pass/fail)
4. `## Compatibility` (supported versions, mixed-version expectation, rollback notes)
5. `## Validation` (commands and expected evidence)
6. `## Risks`

### Validation Evidence Baseline (TiDB Policy-Aligned)

When applicable, attach evidence for:

- `make lint` (required for code changes)
- targeted tests: `go test -run <TestName> -tags=intest,deadlock`
- failpoint decision evidence (why enable/disable is required)
- `make bazel_prepare` trigger check (if Go files moved/added/renamed, Bazel files changed, or go.mod/go.sum changed)

## Backlog Dependency Overview

| Sequence | Issue ID | Type | Dependency Type | Depends On | Unblock When |
|---|---|---|---|---|---|
| 1 | A-EPIC | Epic | Hard | - | roadmap baseline approved |
| 2 | A-1 | Sub-issue | Hard | A-EPIC | epic scope and contract locked |
| 3 | A-2 | Sub-issue | Hard | A-1 | profile bootstrap objects merged |
| 4 | A-3 | Sub-issue | Hard | A-1 | profile baseline merged |
| 5 | A-4 | Sub-issue | Hard | A-3 | tenant enforcement hooks merged |
| 6 | A-5 | Sub-issue | Hard | A-1, A-2, A-3, A-4 | all A foundations merged and testable |
| 7 | B-EPIC | Epic | Hard | A-5 | Milestone A acceptance gate green |
| 8 | B-1 | Sub-issue | Hard | B-EPIC, A-5 | A contracts stable and B epic created |
| 9 | B-2 | Sub-issue | Hard | B-1 | retrieval contract stable |
| 10 | B-3 | Sub-issue | Hard | B-EPIC, A-5 | A gate green and B lifecycle scope approved |
| 11 | B-4 | Sub-issue | Hard | B-1, B-2, B-3 | all B core components merged |
| 12 | B-5 | Sub-issue | Hard | B-4 | e2e integration evidence ready |
| 13 | C-EPIC | Epic | Hard | B-5 | Milestone B acceptance gate green |
| 14 | C-1 | Sub-issue | Hard | C-EPIC, B-5 | B path stable for observability instrumentation |
| 15 | C-2 | Sub-issue | Hard | C-1 | trace/metrics foundation merged |
| 16 | C-3 | Sub-issue | Hard | C-EPIC, B-5 | B contracts stable for adapter conformance |
| 17 | C-4 | Sub-issue | Hard | C-1, C-3 | observability + integration hooks available |
| 18 | C-5 | Sub-issue | Hard | C-2, C-3, C-4 | C production-hardening features merged |

## Milestone A (Foundation Baseline)

### A-EPIC

- **Title**: `agent-memory: Milestone A foundation baseline`
- **Suggested labels**: `type/enhancement`, `component/session`, `component/docs`
- **Parent**: none

```markdown
## Summary
Deliver Milestone A foundation baseline for agent-memory: schema profile v1, fail-closed tenant isolation baseline, and audit baseline.

## Scope
- Standardized memory schema baseline and compatibility views
- Mandatory tenant context enforcement
- Baseline memory audit trail

## Success Criteria
- [x] Milestone A entry/exit criteria in `docs/plans/2026-03-07-agent-memory-milestone-a-detailed-design.md` are fully met
- [x] Sub-issues A-1..A-5 merged with validation evidence
- [x] Milestone A acceptance gate is green

## References
- docs/plans/2026-03-07-agent-memory-milestone-a-detailed-design.md
- docs/plans/2026-03-07-agent-memory-standardized-schema-design.md
- docs/plans/2026-03-07-agent-memory-multi-tenant-security-design.md
```

### A-1

- **Title**: `agent-memory: implement profile v1 bootstrap and version registry`
- **Suggested labels**: `type/enhancement`, `component/session`
- **Parent**: A-EPIC
- **Depends on**: A-EPIC

```markdown
## Summary
Implement profile v1 bootstrap objects and profile version registry for standardized memory schema.

## Scope
- Define canonical schema objects for episodic/semantic/procedural memory
- Add profile version registry metadata table
- Ensure bootstrap is idempotent and rollback-aware

## Acceptance Criteria
- [x] Bootstrap can be applied repeatedly without drift
- [x] Profile version metadata is persisted and queryable
- [x] Schema objects match Milestone A contract

## Validation
- [x] Add targeted tests for bootstrap idempotency and metadata persistence
- [x] Document upgrade/downgrade behavior
```

### A-2

- **Title**: `agent-memory: add compatibility views for stable query surface`
- **Suggested labels**: `type/enhancement`, `component/session`
- **Parent**: A-EPIC
- **Depends on**: A-1

```markdown
## Summary
Add compatibility views that stabilize memory read surfaces across schema evolution.

## Scope
- Create `agent_memory_all`, `agent_memory_active`, and `agent_memory_for_retrieval`
- Document view column contracts and compatibility rules

## Acceptance Criteria
- [x] View outputs match design contracts
- [x] Existing client queries can be mapped to views without breaking semantics
- [x] View behavior is covered by tests

## Validation
- [x] Add tests for multi-type union and archived filtering
- [x] Add migration compatibility test notes
```

### A-3

- **Title**: `agent-memory: enforce mandatory tenant context and fail-closed policy`
- **Suggested labels**: `type/enhancement`, `component/session`, `component/planner`
- **Parent**: A-EPIC
- **Depends on**: A-1

```markdown
## Summary
Enforce tenant/namespace context as mandatory for memory operators with explicit fail-closed behavior.

## Scope
- Add required context checks (`tenant_id`, `namespace`)
- Inject tenant predicates into memory access path
- Return explicit errors for missing/invalid context

## Acceptance Criteria
- [x] Missing tenant context always fails closed
- [x] Planner/executor path cannot bypass tenant predicates
- [x] Behavior is consistent across retrieval and assembly entrypoints

## Validation
- [x] Add functional tests for allow/deny paths
- [x] Add concurrent-session checks for context isolation
```

### A-4

- **Title**: `agent-memory: implement baseline audit trail for memory operations`
- **Suggested labels**: `type/enhancement`, `component/session`
- **Parent**: A-EPIC
- **Depends on**: A-3

```markdown
## Summary
Implement baseline audit schema and event emission for memory operation decisions.

## Scope
- Add audit table and event taxonomy (`read/write/delete/policy_denied`)
- Emit audit rows with low-cardinality metadata
- Define retention defaults

## Acceptance Criteria
- [x] Success and deny events are both audited
- [x] No sensitive payload is stored in default audit rows
- [x] Retention and access guidance documented

## Validation
- [x] Add tests for event emission on all baseline operation classes
- [x] Verify storage/cardinality constraints under load
```

### A-5

- **Title**: `agent-memory: validate Milestone A compatibility, rollback, and backup behavior`
- **Suggested labels**: `type/enhancement`, `component/docs`
- **Parent**: A-EPIC
- **Depends on**: A-1, A-2, A-3, A-4

```markdown
## Summary
Produce evidence that Milestone A can be safely upgraded, rolled back, and backed up/restored.

## Scope
- Mixed-version compatibility checks
- Upgrade/downgrade rehearsal for profile v1
- Backup/restore consistency for profile and audit metadata

## Acceptance Criteria
- [x] Upgrade and rollback rehearsals are documented and reproducible
- [x] Backup/restore consistency checks pass
- [x] Milestone A acceptance gate can be marked complete

## Validation
- [x] Add compatibility test evidence links
- [x] Record exact commands used for rehearsal
```

## Milestone B (Core Retrieval Path)

### B-EPIC

- **Title**: `agent-memory: Milestone B core retrieval path`
- **Suggested labels**: `type/enhancement`, `component/planner`, `component/executor`
- **Parent**: none
- **Depends on**: A-5

```markdown
## Summary
Deliver Milestone B core retrieval path: hybrid retrieval, context assembly, and lifecycle scheduler core.

## Scope
- Hybrid retrieval operator and score fusion
- Context assembly operator with budget control
- Lifecycle scheduler core and transition controls

## Success Criteria
- [x] Sub-issues B-1..B-5 merged with evidence
- [x] End-to-end retrieval->assembly path passes deterministic checks
- [x] Milestone B acceptance gate is green

## References
- docs/plans/2026-03-07-agent-memory-milestone-b-detailed-design.md
```

### B-1

- **Title**: `agent-memory: implement hybrid retrieval operator with deterministic fusion`
- **Suggested labels**: `type/enhancement`, `component/planner`, `component/executor`
- **Parent**: B-EPIC
- **Depends on**: B-EPIC, A-5

```markdown
## Summary
Implement hybrid retrieval operator with deterministic score fusion and bounded candidate processing.

## Scope
- Candidate generation via vector index path
- Hard filter pushdown (`tenant/namespace/metadata/age`)
- Weighted fusion (`vector/recency/importance`) and top-k output

## Acceptance Criteria
- [x] Fixed input and weights always yield deterministic ordering
- [x] Candidate pool is bounded by guardrails
- [x] Missing index path produces explicit fallback warning behavior

## Validation
- [x] Add deterministic ranking tests
- [x] Add fallback-path tests and limit guard tests
```

### B-2

- **Title**: `agent-memory: implement context assembly operator with token budget guarantees`
- **Suggested labels**: `type/enhancement`, `component/executor`
- **Parent**: B-EPIC
- **Depends on**: B-1

```markdown
## Summary
Implement context assembly with deterministic packing, section quotas, and manifest output.

## Scope
- Token estimation and effective-budget computation
- Quota-based packing policy and dropped-candidate reasons
- Final assembled context + manifest emission

## Acceptance Criteria
- [x] Effective budget is never exceeded in successful paths
- [x] Output manifest explains inclusion/exclusion decisions
- [x] Invalid policy/budget paths return explicit errors

## Validation
- [x] Add budget and section policy tests
- [x] Add deterministic packing tests for fixed candidates
```

### B-3

- **Title**: `agent-memory: implement lifecycle scheduler core and transition controls`
- **Suggested labels**: `type/enhancement`, `component/session`
- **Parent**: B-EPIC
- **Depends on**: B-EPIC, A-5

```markdown
## Summary
Implement lifecycle scheduler core with transition execution and admin controls.

## Scope
- Policy-driven transition scheduling
- Transition task execution (`hot/warm/cold/archive`)
- Pause/resume/cancel controls and checkpointed recovery

## Acceptance Criteria
- [x] Lifecycle tasks are idempotent across retries
- [x] Archived state is respected by default retrieval filters
- [x] Control commands are safe under concurrent operations

## Validation
- [x] Add transition correctness tests
- [x] Add failover/recovery tests for checkpoint continuity
```

### B-4

- **Title**: `agent-memory: validate end-to-end retrieval-assembly-lifecycle integration`
- **Suggested labels**: `type/enhancement`, `component/executor`, `component/session`
- **Parent**: B-EPIC
- **Depends on**: B-1, B-2, B-3

```markdown
## Summary
Validate integrated behavior across retrieval, assembly, and lifecycle state transitions.

## Scope
- End-to-end deterministic output checks
- Lifecycle state effect on retrieval eligibility
- Cross-track contract integrity with Milestone A foundations

## Acceptance Criteria
- [x] E2E deterministic scenarios pass
- [x] Lifecycle transitions correctly influence retrieval candidates
- [x] No contract regression against A-level compatibility views

## Validation
- [x] Add integration test suite with representative workloads
- [x] Provide benchmark snapshot vs target budgets
```

### B-5

- **Title**: `agent-memory: complete Milestone B rollout and rollback safety controls`
- **Suggested labels**: `type/enhancement`, `component/docs`
- **Parent**: B-EPIC
- **Depends on**: B-4

```markdown
## Summary
Finalize dark-launch, shadow, limited rollout, and rollback rehearsals for Milestone B.

## Scope
- Feature flag strategy and staged rollout controls
- Independent disable paths for retrieval/assembly/lifecycle
- Operational runbook updates

## Acceptance Criteria
- [x] Rollout stages and rollback steps are tested and documented
- [x] Independent disable paths work without schema rollback
- [x] Milestone B safety gate can be marked complete

## Validation
- [x] Record rollback rehearsal commands and results
- [x] Attach SLO watch results from limited rollout stage
```

## Milestone C (Production Hardening)

### C-EPIC

- **Title**: `agent-memory: Milestone C production hardening`
- **Suggested labels**: `type/enhancement`, `component/session`, `component/docs`
- **Parent**: none
- **Depends on**: B-5

```markdown
## Summary
Deliver Milestone C production hardening: observability/explainability, ecosystem integration contracts, and advanced security hardening.

## Scope
- Trace/metrics + explain surfaces
- Integration adapter contracts and conformance
- Security hardening extensions and operational drills

## Success Criteria
- [x] Sub-issues C-1..C-5 merged with evidence
- [x] Production gate checks pass
- [x] Milestone C acceptance gate is green

## References
- docs/plans/2026-03-07-agent-memory-milestone-c-detailed-design.md
```

### C-1

- **Title**: `agent-memory: implement trace and metrics foundation with bounded overhead`
- **Suggested labels**: `type/enhancement`, `component/session`
- **Parent**: C-EPIC
- **Depends on**: C-EPIC, B-5

```markdown
## Summary
Implement trace and metrics foundations for retrieval and assembly paths with sampling and cost controls.

## Scope
- Trace schema and retention controls
- Metrics for latency, candidate counts, drop ratio, errors
- Sampling/cardinality guardrails

## Acceptance Criteria
- [x] Metrics and traces are emitted correctly in normal and error paths
- [x] Sampling and retention controls cap overhead
- [x] Default outputs avoid sensitive payload capture

## Validation
- [x] Add functional and performance overhead tests
- [x] Add trace-volume stress checks
```

### C-2

- **Title**: `agent-memory: implement explain and trace query surfaces`
- **Suggested labels**: `type/enhancement`, `component/planner`, `component/executor`
- **Parent**: C-EPIC
- **Depends on**: C-1

```markdown
## Summary
Expose explain and trace query surfaces for operator-level and run-level diagnostics.

## Scope
- Plan-level explain surface
- Trace summary query surface
- Slow-log/statement summary extensions for memory paths

## Acceptance Criteria
- [x] Explain output matches actual execution stages
- [x] Trace query output is stable and actionable
- [x] Diagnostic fields are integrated with SQL observability surfaces

## Validation
- [x] Add explain-vs-execution consistency tests
- [x] Add diagnostics correctness tests for error paths
```

### C-3

- **Title**: `agent-memory: deliver adapter operation contract and conformance suite`
- **Suggested labels**: `type/enhancement`, `component/session`, `component/docs`
- **Parent**: C-EPIC
- **Depends on**: C-EPIC, B-5

```markdown
## Summary
Define and validate ecosystem adapter operation contracts (mem0/LangGraph/MCP profiles).

## Scope
- Canonical operation set and error taxonomy
- Version/capability negotiation behavior
- Conformance suite and compatibility matrix

## Acceptance Criteria
- [x] At least one adapter profile passes end-to-end conformance
- [x] Version negotiation behavior is documented and tested
- [x] SQL contract remains source of truth for behavior

## Validation
- [x] Add conformance tests for idempotency and retry behavior
- [x] Add compatibility tests for older profile versions
```

### C-4

- **Title**: `agent-memory: implement security hardening extensions for redaction and purge`
- **Suggested labels**: `type/enhancement`, `component/session`
- **Parent**: C-EPIC
- **Depends on**: C-1, C-3

```markdown
## Summary
Implement security hardening extensions: classification-aware redaction, privileged access audit, and purge workflow controls.

## Scope
- Data classification-aware projection/redaction controls
- Privileged access with explicit audit reason
- Purge workflow with artifact cleanup checkpoints

## Acceptance Criteria
- [x] Redaction behavior is role- and classification-correct
- [x] Purge workflow is auditable and safe under concurrent access
- [x] No cross-tenant leakage in SQL or adapter path

## Validation
- [x] Add security scenario tests and concurrency checks
- [x] Add telemetry safety tests for payload capture policies
```

### C-5

- **Title**: `agent-memory: run production canary and rollback drills for Milestone C`
- **Suggested labels**: `type/enhancement`, `component/docs`
- **Parent**: C-EPIC
- **Depends on**: C-2, C-3, C-4

```markdown
## Summary
Execute production-readiness drills (canary + rollback) and capture evidence for Milestone C acceptance.

## Scope
- Internal canary, controlled pilot, enforce-stage checks
- Rollback drills across observability/integration/security hardening
- Incident and runbook validation

## Acceptance Criteria
- [x] Canary and pilot SLO criteria are met
- [x] Rollback drills complete without unresolved red risks
- [x] Milestone C production gate is marked complete

## Validation
- [x] Attach canary metrics and trace evidence
- [x] Attach rollback drill logs and runbook revisions
```

## Suggested Parent/Epic Creation Order

1. Create `A-EPIC`, then `A-1..A-5`.
2. After A gate is complete, create `B-EPIC`, then `B-1..B-5`.
3. After B gate is complete, create `C-EPIC`, then `C-1..C-5`.

## References

- `docs/plans/2026-03-07-agent-memory-design-overview-index.md`
- `docs/plans/2026-03-07-agent-memory-milestone-a-detailed-design.md`
- `docs/plans/2026-03-07-agent-memory-milestone-b-detailed-design.md`
- `docs/plans/2026-03-07-agent-memory-milestone-c-detailed-design.md`
