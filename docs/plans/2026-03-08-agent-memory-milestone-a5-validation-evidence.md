# TiDB Agent Memory Milestone A-5 Validation Evidence

- Author(s): TBD
- Date: 2026-03-08
- Scope: Milestone A-5 (`agent-memory: validate Milestone A compatibility, rollback, and backup behavior`)

## 1. Objective

This document captures reproducible local validation evidence for Milestone A-5.
It maps the A-5 acceptance criteria to exact test commands and observed results.

Primary contract reference:

- `docs/plans/2026-03-07-agent-memory-milestone-issue-backlog.md` (A-5 block)
- `docs/plans/2026-03-07-agent-memory-milestone-a-detailed-design.md` (Acceptance Gate)

## 2. Code Window Under Validation

Agent-memory foundation commits validated in this round (A-1 to A-4 closure surface):

1. `3fcdd8ab4c` session: initialize default agent-memory profile registry row
2. `cc687522d5` session,meta: add baseline agent-memory memory tables
3. `b7f625289f` session,meta: add agent-memory compatibility views
4. `56fbcb4ed0` session: validate agent-memory compatibility view bootstrap semantics
5. `f1ebfc98b4` planner,session: enforce agent-memory tenant-context fail-closed checks
6. `b9a1dfb585` planner,session: inject tenant filters for agent-memory reads
7. `86ab9f2610` planner: enforce agent-memory context checks on EXECUTE
8. `d4e74af84b` planner,session: protect profile-version table with tenant checks
9. `247944c523` session,meta: add baseline agent-memory audit table
10. `61df7a247d` planner,session: enforce tenant checks on memory audit table
11. `82967b6264` planner,session: audit policy-denied tenant-context checks
12. `c9677dafc9` planner,session: audit read/write/delete memory actions

## 3. Acceptance Criteria Mapping (A-5)

| A-5 Acceptance Item | Evidence | Status |
|---|---|---|
| Upgrade and rollback rehearsals are documented and reproducible | `TestUpgrade` and `TestBootstrap` execute downgrade + re-bootstrap path and verify agent-memory tables/views and fail-closed behavior | PASS (local) |
| Backup/restore consistency checks pass | `TestVersionedBootstrapSchemas` validates versioned bootstrap schema definitions and IDs; `TestUpgrade` validates recreated objects after downgrade | PASS (local bootstrap simulation) |
| Milestone A acceptance gate can be marked complete | A-1..A-4 behavior is covered by targeted tests listed below; this doc records replayable commands and outcomes | PASS (local evidence captured) |

## 4. Reproducible Validation Commands

Failpoint decision evidence:

- `pkg/session` and `pkg/planner/core` include failpoint/testfailpoint usage.
- Tests were run with explicit failpoint enable/disable flow.

Executed command:

```bash
make failpoint-enable && (
  go test ./pkg/session -run 'TestBootstrap$|TestUpgrade$|TestVersionedBootstrapSchemas$' -tags=intest,deadlock &&
  go test ./pkg/planner/core -run TestAgentMemoryTenantContextFailClosed -tags=intest,deadlock &&
  go test ./pkg/planner/core/tests/prepare -run TestAgentMemoryPrepareExecuteFailClosed -tags=intest,deadlock &&
  go test ./pkg/sessionctx/variable -run TestAgentMemoryContextSysVars -tags=intest,deadlock
); rc=$?; make failpoint-disable; exit $rc
```

Observed output summary:

- `ok   github.com/pingcap/tidb/pkg/session`
- `ok   github.com/pingcap/tidb/pkg/planner/core`
- `ok   github.com/pingcap/tidb/pkg/planner/core/tests/prepare`
- `ok   github.com/pingcap/tidb/pkg/sessionctx/variable`

## 5. Compatibility Notes

Covered by this validation set:

1. Bootstrap object existence and idempotency for profile/version, memory tables, compatibility views, and audit table.
2. Upgrade rehearsal by downgrading bootstrap metadata and re-running bootstrap initialization.
3. Rollback compatibility window through drop-and-recreate simulation in upgrade tests.
4. Tenant fail-closed and audit emission across direct SQL and prepared execution.

## 6. Risks and Follow-up

Residual risk:

- This evidence is local/unit-level and focuses on bootstrap/upgrade simulation.

Recommended follow-up before external release gate:

1. Add BR/TiCDC workflow-level validation evidence if release checklist requires full backup/restore pipeline proof.
2. Attach CI links for the same targeted test set in PR/issue tracking.
