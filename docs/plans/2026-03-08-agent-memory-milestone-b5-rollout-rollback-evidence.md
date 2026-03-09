# TiDB Agent Memory Milestone B-5 Rollout and Rollback Evidence

- Author(s): TBD
- Date: 2026-03-08
- Scope: Milestone B-5 (`agent-memory: complete Milestone B rollout and rollback safety controls`)

## 1. Objective

This document records the rollout/rollback safety rehearsal evidence for Milestone B core paths.
It maps B-5 acceptance criteria to concrete controls, commands, and observed outcomes.

Contract references:

- `docs/plans/2026-03-07-agent-memory-milestone-issue-backlog.md` (`B-5`)
- `docs/plans/2026-03-07-agent-memory-milestone-b-detailed-design.md` (rollout/rollback and safety gates)

## 2. Feature-Flag Control Surface

Independent controls exercised in this rehearsal:

1. `tidb_enable_agent_memory_hybrid_retrieval`
2. `tidb_enable_agent_memory_context_assembly`
3. `tidb_enable_agent_memory_lifecycle_scheduler`

All controls are session/global-gated and do not require schema rollback.

## 3. Rollout Stages and Control Strategy

### Stage A: Dark launch

- Keep all B flags `OFF` (default-safe mode).
- Verify SQL paths still operate under A-level tenant guardrails.

### Stage B: Shadow validation

- Enable `hybrid_retrieval` only for scoped sessions.
- Validate deterministic ranking/packing outputs and warning behavior.

### Stage C: Limited rollout

- Enable `hybrid_retrieval + context_assembly` for pilot tenants.
- Keep lifecycle scheduler under explicit control with pause/resume/cancel safety.

### Stage D: Broad rollout

- Enable all B flags with SLO watch and rollback readiness.

## 4. Rollback Rehearsal Matrix

| Scenario | Command/Test Surface | Expected Outcome | Local Result |
|---|---|---|---|
| Disable HR only | `set @@tidb_enable_agent_memory_hybrid_retrieval='OFF'` + retrieval query | No fallback warning, schema unchanged | PASS |
| Enable HR only | `set @@tidb_enable_agent_memory_hybrid_retrieval='ON'` + vector-distance `ORDER BY ... LIMIT` retrieval query | Fallback warning emitted when optimized index path is unavailable for vector retrieval shape | PASS |
| Enable HR with constant-distance sort | `set @@tidb_enable_agent_memory_hybrid_retrieval='ON'` + constant-distance `ORDER BY ... LIMIT` query | No fallback warning for non-embedding constant-distance expression | PASS |
| Enable HR with set-operator vector retrieval | `set @@tidb_enable_agent_memory_hybrid_retrieval='ON'` + `UNION ALL ... ORDER BY vec_l2_distance(embedding, ...) LIMIT` | Fallback warning emitted once for set-operator retrieval shape | PASS |
| Disable CA independently | `set @@tidb_enable_agent_memory_context_assembly='OFF'` | No schema rollback required | PASS |
| Disable LC independently | `set @@tidb_enable_agent_memory_lifecycle_scheduler='OFF'` | Scheduler disabled without touching schema/profile tables | PASS |
| Control safety under concurrency | lifecycle pause/resume/cancel tests | Canceled jobs reject further execution safely | PASS |

## 5. Validation Commands and Results

Failpoint decision evidence:

- `pkg/planner/core`, `pkg/session`, and `pkg/sessionctx/variable` test surfaces use failpoint/testfailpoint patterns.
- Validation was run with explicit failpoint enable/disable flow.

Executed commands:

```bash
make failpoint-enable && (
  go test ./pkg/planner/core -run 'TestAssembleAgentMemoryContextBudgetAndManifest|TestAssembleAgentMemoryContextSectionQuota|TestAssembleAgentMemoryContextDeterministicPacking|TestAssembleAgentMemoryContextInvalidPolicy|TestAssembleAgentMemoryContextCandidateCountBound|TestRunAgentMemoryPipelineDeterministicOutput|TestRunAgentMemoryPipelineLifecycleStateInfluence|TestAgentMemoryTenantContextFailClosed|TestAgentMemoryRetrievalFallbackWarning|TestAgentMemoryRetrievalFallbackWarningOnExecute|TestAgentMemoryRetrievalFallbackWarningOnExecuteVectorDistance|TestAgentMemoryRetrievalFallbackWarningNotForConstantDistance|TestAgentMemoryRetrievalFallbackWarningDeduplicated|TestAgentMemoryRetrievalFallbackWarningOnSetOpr|TestAgentMemoryRetrievalFallbackWarningOnSetOprConstantDistance|TestRankAgentMemoryCandidatesDeterministic|TestRankAgentMemoryCandidatesBoundedByTopK|TestRankAgentMemoryCandidatesWeightValidation|TestRankAgentMemoryCandidatesNormalizeScores' -tags=intest,deadlock &&
  go test ./pkg/sessionctx/variable -run 'TestAgentMemoryContextSysVars|TestAgentMemoryHybridRetrievalSysVars' -tags=intest,deadlock &&
  go test ./pkg/session -run 'TestAgentMemoryLifecycleTaskIdempotenceAcrossRetries|TestAgentMemoryLifecyclePauseResumeCancelConcurrentSafe|TestAgentMemoryLifecycleCheckpointRecovery|TestAgentMemoryLifecycleFilterArchivedDefault|TestBootstrap$|TestUpgrade$|TestVersionedBootstrapSchemas$' -tags=intest,deadlock
); rc=$?; make failpoint-disable; exit $rc

go test ./br/pkg/restore/snap_client -run TestMonitorTheSystemTableIncremental -tags=intest,deadlock

unset CI && GOPROXY=https://goproxy.cn,direct make bazel_prepare
GOPROXY=https://goproxy.cn,direct make lint
```

Observed outcome summary:

- All listed planner/session/sessionctx/BR targeted tests passed.
- `bazel_prepare` succeeded with local non-CI mode and updated Bazel metadata for new Go files.
- `make lint` passed.

### Limited-rollout SLO watch snapshot (local)

This snapshot is a local limited-rollout watch check for latency and allocation stability, not production dashboard proof.

Command used:

```bash
make failpoint-enable && go test ./pkg/planner/core -run '^$' -bench 'BenchmarkRunAgentMemoryPipeline|BenchmarkObserveAgentMemoryPipeline' -benchmem -tags=intest,deadlock -count=1; rc=$?; make failpoint-disable; exit $rc
```

Observed local snapshot values (darwin/arm64):

- `BenchmarkRunAgentMemoryPipeline/N=32`: `7724 ns/op`, `23104 B/op`, `24 allocs/op`
- `BenchmarkRunAgentMemoryPipeline/N=128`: `22042 ns/op`, `59584 B/op`, `24 allocs/op`
- `BenchmarkRunAgentMemoryPipeline/N=512`: `103698 ns/op`, `188480 B/op`, `24 allocs/op`
- `BenchmarkObserveAgentMemoryPipeline/N=32`: `7535 ns/op`, `23104 B/op`, `24 allocs/op`
- `BenchmarkObserveAgentMemoryPipeline/N=128`: `22286 ns/op`, `59584 B/op`, `24 allocs/op`
- `BenchmarkObserveAgentMemoryPipeline/N=512`: `104156 ns/op`, `188480 B/op`, `24 allocs/op`

## 6. Acceptance Criteria Mapping (B-5)

| B-5 Acceptance Item | Evidence | Status |
|---|---|---|
| Rollout stages and rollback steps are tested and documented | Stage strategy + rehearsal matrix + exact commands in this doc | PASS |
| Independent disable paths work without schema rollback | HR/CA/LC independent flag controls verified in tests and rehearsal | PASS |
| Milestone B safety gate can be marked complete | Local rehearsal scope only: targeted failpoint-wrapped safety control bundle passed in this run (`TestAgentMemoryTenantContextFailClosed`, retrieval fallback warning suite, and session lifecycle safety suite); production SLO and full SQL/planner/executor contract closure remain tracked separately | PASS (local rehearsal scope) |

## 7. Residual Risks and Follow-up

Residual risks:

1. This rehearsal is local-targeted and not a full production traffic replay.
2. SLO watch metrics for limited rollout are represented by local test outcomes, not full tenant traffic dashboards.

Recommended follow-up:

1. Attach CI links for the same test surfaces in B-EPIC tracking.
2. During real limited rollout, append SLO snapshots (latency/error budgets) to this evidence document.
