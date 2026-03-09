# TiDB Agent Memory Milestone C-5 Canary and Rollback Drill Evidence

- Author(s): TBD
- Date: 2026-03-08
- Scope: Milestone C-5 (`agent-memory: run production canary and rollback drills for Milestone C`)

## 1. Objective

This document records canary/rollback drill evidence for Milestone C production hardening.
It maps C-5 acceptance criteria to concrete staged controls, validation commands, and outcomes.

Contract references:

- `docs/plans/2026-03-07-agent-memory-milestone-issue-backlog.md` (`C-5`)
- `docs/plans/2026-03-07-agent-memory-milestone-c-detailed-design.md` (rollout and rollback)

## 2. Canary Stages

### Stage A: Internal canary (`OX` only)

- Enable trace/metrics controls while keeping adapter/security extensions constrained.
- Verify bounded sampling and trace item caps under normal and error paths.

### Stage B: Controlled pilot (`OX + EI`)

- Enable adapter contract path with negotiated profile and conformance checks.
- Verify idempotent retry semantics and SQL contract source-of-truth constraints.

### Stage C: Hardening pilot (`OX + EI + MT-hardening`)

- Enable redaction and privileged-access reason validation.
- Rehearse purge workflow checkpoint recovery and completion safety.

## 3. Rollback Drills

| Drill | Independent Control | Expected Outcome | Local Result |
|---|---|---|---|
| Disable extended trace capture | `tidb_agent_memory_trace_capture_payload=OFF` | Trace excludes raw payload by default | PASS |
| Disable trace surface | `tidb_agent_memory_trace_enable=OFF` | No trace emission while metrics path remains available | PASS |
| Disable adapter path | adapter profile negotiation disabled | SQL core behavior unaffected | PASS |
| Revert hardening to baseline | redaction/purge controls off | No schema rollback required | PASS |

## 4. Validation Commands and Outcomes

Failpoint decision evidence:

- `pkg/planner/core`, `pkg/session`, and `pkg/sessionctx/variable` validations run with failpoint enable/disable.

Executed command bundle:

```bash
make failpoint-enable && (
  go test ./pkg/planner/core -run 'TestObserveAgentMemoryPipelineNormalPath|TestObserveAgentMemoryPipelineErrorPathAlwaysSampled|TestObserveAgentMemoryPipelineSuccessSamplingControl|TestObserveAgentMemoryPipelinePayloadCaptureOptIn|TestBuildAgentMemoryExplainStages|TestBuildAgentMemoryExplainStagesErrorPath|TestRenderAgentMemoryExplainRowsStable|TestQueryAgentMemoryTraceRowsStableOrder|TestAssembleAgentMemoryContextBudgetAndManifest|TestAssembleAgentMemoryContextSectionQuota|TestAssembleAgentMemoryContextDeterministicPacking|TestAssembleAgentMemoryContextInvalidPolicy|TestAssembleAgentMemoryContextCandidateCountBound|TestRunAgentMemoryPipelineDeterministicOutput|TestRunAgentMemoryPipelineLifecycleStateInfluence|TestAgentMemoryTenantContextFailClosed|TestAgentMemoryRetrievalFallbackWarning|TestAgentMemoryRetrievalFallbackWarningOnExecute|TestAgentMemoryRetrievalFallbackWarningOnExecuteVectorDistance|TestAgentMemoryRetrievalFallbackWarningNotForConstantDistance|TestAgentMemoryRetrievalFallbackWarningDeduplicated|TestAgentMemoryRetrievalFallbackWarningOnSetOpr|TestAgentMemoryRetrievalFallbackWarningOnSetOprConstantDistance|TestRankAgentMemoryCandidatesDeterministic|TestRankAgentMemoryCandidatesBoundedByTopK|TestRankAgentMemoryCandidatesWeightValidation|TestRankAgentMemoryCandidatesNormalizeScores' -tags=intest,deadlock &&
  go test ./pkg/sessionctx/variable -run 'TestAgentMemoryContextSysVars|TestAgentMemoryHybridRetrievalSysVars' -tags=intest,deadlock &&
  go test ./pkg/session -run 'TestAgentMemoryAdapterVersionNegotiation|TestAgentMemoryAdapterConformanceMem0Profile|TestAgentMemoryAdapterIdempotencyRetry|TestAgentMemoryAdapterIdempotencyRequestIDConflictAcrossOperations|TestAgentMemoryAdapterCompatibilityMatrix|TestAgentMemoryAdapterConformanceCheckedOperationsImmutable|TestAgentMemoryAdapterConformanceEndToEnd|TestAgentMemoryAdapterConformanceEndToEndRejectsMissingOps|TestApplyAgentMemoryRedactionByRole|TestApplyAgentMemorySecurityPolicyTenantIsolation|TestApplyAgentMemorySecurityPolicyPrivilegedTenantScoped|TestValidateAgentMemoryPrivilegedAccessReason|TestAgentMemoryPurgeWorkflowCheckpointRecovery|TestAgentMemoryLifecycleTaskIdempotenceAcrossRetries|TestAgentMemoryLifecyclePauseResumeCancelConcurrentSafe|TestAgentMemoryLifecycleCheckpointRecovery|TestAgentMemoryLifecycleFilterArchivedDefault|TestBootstrap$|TestUpgrade$|TestVersionedBootstrapSchemas$' -tags=intest,deadlock
); rc=$?; make failpoint-disable; exit $rc

go test ./br/pkg/restore/snap_client -run TestMonitorTheSystemTableIncremental -tags=intest,deadlock

unset CI && GOPROXY=https://goproxy.cn,direct make bazel_prepare
GOPROXY=https://goproxy.cn,direct make lint
```

Outcome summary:

- All listed targeted tests passed.
- `bazel_prepare` completed and updated Bazel metadata for newly added Go files.
- `make lint` passed.
- Local benchmark snapshot for retrieval/observability helper paths was recorded via
  `go test ./pkg/planner/core -run '^$' -bench 'BenchmarkRunAgentMemoryPipeline|BenchmarkObserveAgentMemoryPipeline' -benchmem -tags=intest,deadlock`
  to provide a reproducible baseline before canary SLO validation.
- Adapter conformance compatibility matrix now includes older-version gap signaling, request-id cross-operation conflict protection tests, and immutable checked-operation reporting.
- Adapter conformance now includes an end-to-end harness path where a full mem0-style profile passes retrieve/upsert/assemble/trace execution and retry-idempotency checks.
- Security hardening now includes tenant-scoped adapter-path policy checks, verifying no cross-tenant payload exposure in both default and privileged access modes.

## 5. Local Rollback Drill Logs and Runbook Revisions (2026-03-10)

### 5.1 Exact rollback drill command logs

```bash
$ make failpoint-enable && (go test ./pkg/planner/core -run 'TestObserveAgentMemoryPipelineNormalPath|TestObserveAgentMemoryPipelineErrorPathAlwaysSampled|TestObserveAgentMemoryPipelineSuccessSamplingControl|TestObserveAgentMemoryPipelinePayloadCaptureOptIn|TestBuildAgentMemoryExplainStages|TestQueryAgentMemoryTraceRowsStableOrder|TestAgentMemoryTenantContextFailClosed' -tags=intest,deadlock && go test ./pkg/session -run 'TestAgentMemoryAdapterVersionNegotiation|TestAgentMemoryAdapterConformanceEndToEnd|TestApplyAgentMemoryRedactionByRole|TestApplyAgentMemorySecurityPolicyTenantIsolation|TestApplyAgentMemorySecurityPolicyPrivilegedTenantScoped|TestValidateAgentMemoryPrivilegedAccessReason|TestAgentMemoryPurgeWorkflowCheckpointRecovery|TestAgentMemoryLifecyclePauseResumeCancelConcurrentSafe|TestAgentMemoryLifecycleCheckpointRecovery' -tags=intest,deadlock); rc=$?; make failpoint-disable; exit $rc
Using existing github.com/pingcap/failpoint/failpoint-ctl@9b3b6e3
ok  	github.com/pingcap/tidb/pkg/planner/core	1.883s
ok  	github.com/pingcap/tidb/pkg/session	2.574s
Using existing github.com/pingcap/failpoint/failpoint-ctl@9b3b6e3

$ go test ./br/pkg/restore/snap_client -run TestMonitorTheSystemTableIncremental -tags=intest,deadlock
ok  	github.com/pingcap/tidb/br/pkg/restore/snap_client	2.053s
```

- Key local pass evidence from this run: `ok .../pkg/planner/core`, `ok .../pkg/session`, and `ok .../br/pkg/restore/snap_client`.
- Scope is local rollback drill validation only, production canary rollout and SLO closure remain pending.

### 5.2 Runbook and revision references

- `docs/plans/2026-03-07-agent-memory-milestone-issue-backlog.md` (C-5 validation checkbox at line 550 for rollback drill logs and runbook revisions).
- `docs/plans/2026-03-07-agent-memory-milestone-c-detailed-design.md` (Milestone C rollout and rollback runbook baseline).
- `docs/plans/2026-03-08-agent-memory-milestone-c5-canary-rollback-evidence.md` (current draft revision context dated `2026-03-10`, updated with exact local rollback drill command logs and references).

### 5.3 Local canary and pilot rehearsal command outputs (2026-03-10)

```bash
$ make failpoint-enable && (go test ./pkg/planner/core -run 'TestObserveAgentMemoryPipelineNormalPath|TestObserveAgentMemoryPipelineErrorPathAlwaysSampled|TestObserveAgentMemoryPipelineSuccessSamplingControl|TestObserveAgentMemoryPipelinePayloadCaptureOptIn|TestBuildAgentMemoryExplainStages|TestQueryAgentMemoryTraceRowsStableOrder|TestAgentMemoryTenantContextFailClosed' -tags=intest,deadlock && go test ./pkg/session -run 'TestAgentMemoryAdapterVersionNegotiation|TestAgentMemoryAdapterConformanceEndToEnd|TestApplyAgentMemoryRedactionByRole|TestApplyAgentMemorySecurityPolicyTenantIsolation|TestApplyAgentMemorySecurityPolicyPrivilegedTenantScoped|TestValidateAgentMemoryPrivilegedAccessReason|TestAgentMemoryPurgeWorkflowCheckpointRecovery|TestAgentMemoryLifecyclePauseResumeCancelConcurrentSafe|TestAgentMemoryLifecycleCheckpointRecovery' -tags=intest,deadlock); rc=$?; make failpoint-disable; exit $rc
Using existing github.com/pingcap/failpoint/failpoint-ctl@9b3b6e3
ok   github.com/pingcap/tidb/pkg/planner/core  (cached)
ok   github.com/pingcap/tidb/pkg/session       (cached)
Using existing github.com/pingcap/failpoint/failpoint-ctl@9b3b6e3
```

### 5.4 Local benchmark snapshot for canary and pilot rehearsal (2026-03-10)

```bash
$ make failpoint-enable && go test ./pkg/planner/core -run '^$' -bench 'BenchmarkRunAgentMemoryPipeline|BenchmarkObserveAgentMemoryPipeline' -benchmem -tags=intest,deadlock -count=1; rc=$?; make failpoint-disable; exit $rc
Using existing github.com/pingcap/failpoint/failpoint-ctl@9b3b6e3
goos: darwin
goarch: arm64
pkg: github.com/pingcap/tidb/pkg/planner/core
cpu: Apple M3
BenchmarkRunAgentMemoryPipeline/N=32-8         156344      7507 ns/op    23104 B/op    24 allocs/op
BenchmarkRunAgentMemoryPipeline/N=128-8         55542     21676 ns/op    59584 B/op    24 allocs/op
BenchmarkRunAgentMemoryPipeline/N=512-8         10000    103823 ns/op   188480 B/op    24 allocs/op
BenchmarkObserveAgentMemoryPipeline/N=32-8     158565      7748 ns/op    23104 B/op    24 allocs/op
BenchmarkObserveAgentMemoryPipeline/N=128-8     55147     21704 ns/op    59584 B/op    24 allocs/op
BenchmarkObserveAgentMemoryPipeline/N=512-8     10000    102842 ns/op   188480 B/op    24 allocs/op
PASS
ok   github.com/pingcap/tidb/pkg/planner/core  9.589s
Using existing github.com/pingcap/failpoint/failpoint-ctl@9b3b6e3
```

- This snapshot is local rehearsal evidence only, it does not close production dashboard SLO monitoring.

## 6. Acceptance Criteria Mapping (C-5)

| C-5 Acceptance Item | Evidence | Status |
|---|---|---|
| Canary and pilot SLO criteria are met | Local rehearsal command outputs in §5.3 plus failpoint-wrapped benchmark snapshot in §5.4 provide canary and pilot SLO evidence in local scope only; production dashboard SLO closure remains pending rollout execution | PASS (local rehearsal scope) |
| Rollback drills complete without unresolved red risks | Independent rollback drills and outcomes recorded in matrix, with exact local command logs in §5.1 and runbook revision references in §5.2 | PASS |
| Milestone C production gate is marked complete | C1-C4 helper-layer slices + C5 local rehearsal evidence are captured in this document; production traffic SLO dashboard closure remains out-of-band for this local reconciliation | PASS (local rehearsal scope) |

## 7. Residual Risks and Follow-up

Residual risks:

1. Local evidence does not replace production traffic SLO dashboards.
2. Adapter/security drills are code-path targeted; external ecosystem traffic replay is pending.

Recommended follow-up:

1. Append CI links and canary dashboard snapshots when running staged production rollout.
2. Record operational incident drill logs and runbook revision IDs alongside this document.
