# TiDB Agent Memory Milestone B Detailed Design (Core Retrieval Path)

- Author(s): TBD
- Discussion PR: TBD
- Tracking Issue: TBD

## 1. Objective and Contract

Milestone B delivers the core execution path for agent-memory workloads:

1. hybrid retrieval (`HR`)
2. context assembly (`CA`)
3. lifecycle engine core (`LC`)

The contract is to provide deterministic retrieval and token-bounded context output under
production-safe guardrails, while preserving compatibility established in Milestone A.

## 2. Entry and Exit Criteria

### Entry Criteria

1. Milestone A acceptance gates are all green.
2. Schema/profile + tenant enforcement baseline are active.
3. Feature flags exist for each Milestone B capability.

### Exit Criteria

1. Hybrid retrieval function/operator passes deterministic ranking tests.
2. Context assembly enforces token budget and section quotas.
3. Lifecycle scheduler can execute transition jobs with pause/resume/cancel control.
4. End-to-end path (`retrieve -> assemble`) validates under tenant isolation and compatibility checks.

## 3. Scope

### In Scope

- Retrieval score fusion and candidate management.
- Context assembly with estimator and manifest output.
- Lifecycle transitions and retention actions (core subset).

### Out of Scope

- Full observability and explain API surfaces (Milestone C).
- Ecosystem adapter contracts and SDK profiles (Milestone C).
- Advanced security hardening beyond baseline enforcement.

## 4. Detailed Design

### 4.1 Hybrid Retrieval (`HR`)

Retrieval path responsibilities:

1. candidate generation from vector index
2. hard predicate filtering (`tenant`, `namespace`, metadata, age)
3. weighted score fusion (`vector`, `recency`, `importance`)
4. top-k output with score decomposition

Core invariants:

- deterministic order for same input and same weights
- bounded candidate set by configurable `candidate_n`
- explicit warning path when index-based optimization is unavailable

Guardrails:

- reject invalid weights and invalid vector dimensions
- enforce per-tenant query budget limits for candidate expansion

### 4.2 Context Assembly (`CA`)

Assembly path responsibilities:

1. receive retrieval candidates
2. estimate token cost per candidate
3. apply section quotas and packing strategy
4. emit assembled context + manifest + dropped reasons

Core invariants:

- estimated context input does not exceed effective budget
- deterministic selection under fixed candidate set and policy
- output manifest can explain why each item was included/excluded

Guardrails:

- error on non-positive effective budget
- validate section-policy shape before assembly begins
- bound candidate and item counts to avoid memory blowups

### 4.3 Lifecycle Core (`LC`)

Lifecycle core responsibilities:

1. policy-driven transition scheduling
2. transition execution for hot/warm/cold/archive
3. optional delete based on policy flag
4. checkpointed progress and failover-safe job continuation

Core invariants:

- lifecycle actions are idempotent by job/task identity
- archived memory is excluded from default retrieval path
- policy updates do not break running job safety

Guardrails:

- default safe mode avoids hard delete until explicitly enabled
- max concurrent lifecycle jobs per physical table

## 5. End-to-End Data Flow Contract

```text
Query -> Hybrid Retrieval -> Candidate Set -> Context Assembly -> Context Pack
                           \-> Lifecycle policy affects candidate eligibility via state filters
```

Cross-track contracts enforced in Milestone B:

- state semantics from lifecycle are consumed by retrieval/assembly.
- tenant isolation rules from Milestone A apply to every operator path.
- profile view contracts remain stable for caller SQL.

## 6. Dependency Ordering

Hard dependencies in Milestone B:

1. retrieve operator must stabilize before context assembly final tuning.
2. lifecycle state filtering must integrate before retrieval performance validation.
3. retrieval and assembly feature flags must remain independently controllable.

Execution sequence recommendation:

1. `HR` core logic and tests
2. `CA` integration over stable retrieval output
3. `LC` scheduler and transition integration
4. end-to-end and performance validation

## 7. Work Packages

### WP-B1: Hybrid Retrieval Operator

Outputs:

- operator contract and score-fusion behavior
- fallback semantics for non-index path
- deterministic ranking test coverage

### WP-B2: Context Assembly Operator

Outputs:

- token estimation contract and budget guardrails
- section policy and manifest contract
- deterministic packing tests

### WP-B3: Lifecycle Scheduler Core

Outputs:

- lifecycle policy execution state machine
- checkpointing and failure recovery semantics
- admin controls for pause/resume/cancel

### WP-B4: E2E Path Validation

Outputs:

- integrated retrieval-to-assembly verification
- lifecycle impact validation on retrieval eligibility
- baseline performance report vs target budgets

## 8. Validation Plan

### Functional Validation

- retrieval score decomposition correctness
- context budget/manifest correctness
- lifecycle transition and control command correctness

### Integration Validation

- end-to-end deterministic context output for fixed inputs
- lifecycle state changes reflected in retrieval candidates
- tenant-policy consistency across all B-path operators

### Performance Validation

- retrieval latency under candidate pool matrix
- assembly latency under candidate-size matrix
- scheduler overhead under table/job scale matrix

### Compatibility Validation

- no schema contract break against Milestone A views
- mixed-mode feature-flag operation with safe fallback

## 9. Rollout and Rollback

### Rollout Stages

1. dark launch: operators available behind disabled flags.
2. shadow queries: retrieve/assemble outputs validated out-of-band.
3. limited tenant rollout: strict SLO watch.
4. broad rollout with guarded defaults.

### Rollback Rules

- disable `HR`/`CA` flags independently to revert to app-side path.
- disable lifecycle execution while keeping policy metadata intact.
- maintain compatibility views and schema profile from Milestone A.

## 10. Risks and Mitigations

1. Risk: score fusion defaults harm recall quality in real workloads.
   - Mitigation: conservative defaults + workload benchmark matrix before broad rollout.
2. Risk: lifecycle transitions remove useful context unexpectedly.
   - Mitigation: safe default (`delete off`) + dry-run output validation.
3. Risk: assembly estimator variance causes occasional overflow.
   - Mitigation: configurable margin ratio and strict error path.
4. Risk: high candidate counts produce CPU spikes.
   - Mitigation: hard candidate caps and per-tenant rate controls.

## 11. Acceptance Gate for Milestone B

Milestone B is accepted only when all gates pass:

1. Integration gate: `HR + CA + LC` end-to-end path is deterministic and stable.
2. Performance gate: latency and overhead are within agreed targets.
3. Compatibility gate: Milestone A contracts remain intact.
4. Safety gate: independent rollback controls validated in rehearsal.
