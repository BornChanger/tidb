# TiDB Agent Memory Milestone C Detailed Design (Production Hardening)

- Author(s): TBD
- Discussion PR: TBD
- Tracking Issue: TBD

## 1. Objective and Contract

Milestone C hardens the agent-memory stack for production use at scale. The core contract is to
complete observability/explainability, ecosystem integration, and advanced security hardening
without regressing behavior delivered in Milestones A and B.

### In Scope

- Observability and explainability surfaces (`OX`)
- Ecosystem integration contracts and adapter profiles (`EI`)
- Security hardening extensions on top of Milestone A baseline (`MT-hardening`)

### Out of Scope

- New foundational schema model redesign
- Replacement of Milestone B operator contracts
- Vendor-specific framework lock-in behavior

## 2. Entry and Exit Criteria

### Entry Criteria

1. Milestone B end-to-end core path is accepted.
2. Feature flags for observability and integrations are available.
3. Baseline tenant isolation and schema profile are in enforce mode.

### Exit Criteria

1. Explain and trace surfaces are available with bounded overhead.
2. At least one adapter profile validates end-to-end with conformance tests.
3. Security hardening controls (redaction/audit/deletion workflow) pass acceptance checks.
4. Canary rollout and rollback drills are completed with evidence.

## 3. Detailed Design

### 3.1 Observability and Explainability (`OX`)

Required surfaces:

1. plan-level explain for memory retrieval/assembly paths
2. trace-level execution summary for completed operations
3. metrics for latency, candidate volume, drop ratio, and error classes

Required constraints:

- tracing must be sampleable and bounded
- default traces avoid sensitive payload leakage
- explain outputs should align with actual execution stages

Operational outputs:

- dashboard-ready metrics
- slow-log/statement-summary extensions for memory operators
- trace retention policy and cleanup controls

### 3.2 Ecosystem Integrations (`EI`)

Integration contract model:

- canonical operation set over TiDB memory capabilities
- strict idempotency via request-id semantics
- explicit capability/version negotiation for adapters

Initial profile set:

1. mem0-style memory interaction profile
2. LangGraph store/search/checkpoint profile
3. MCP tool interface profile for retrieval/upsert/context pack/trace

Design constraints:

- SQL contracts remain source of truth
- adapters provide protocol translation, not core behavior changes
- compatibility matrix governs profile evolution

### 3.3 Security Hardening Extensions (`MT-hardening`)

Hardening controls in C:

1. data classification-aware redaction policies
2. privileged access workflow with explicit audit reason
3. deletion and purge workflow with artifact cleanup checkpoints
4. telemetry safety controls for payload capture opt-in

Security invariants:

- no cross-tenant data leakage through trace, cache, or adapter path
- read/write controls are enforced consistently across SQL and adapter access

## 4. Dependency Ordering

Hard dependencies in Milestone C:

1. observability primitives should land before broad adapter rollout.
2. adapter conformance requires stable Milestone B behavior contracts.
3. security hardening must cover both SQL-native and adapter-mediated access.

Execution sequence recommendation:

1. implement OX core metrics and trace schema
2. implement explain/trace surfaces
3. implement EI adapter contract and conformance harness
4. implement MT hardening and cross-path security verification
5. run canary and rollback rehearsals

## 5. Work Packages

### WP-C1: Trace and Metrics Foundation

Outputs:

- trace schema and sampling controls
- low-overhead metrics and cardinality guardrails

### WP-C2: Explain and Trace Access Surfaces

Outputs:

- explain command/output contract
- trace query surface for diagnostics

### WP-C3: Adapter Contract and Conformance

Outputs:

- canonical operation contract
- profile/version negotiation behavior
- conformance test suite and compatibility matrix

### WP-C4: Security Hardening

Outputs:

- redaction policy behavior and privilege model
- purge workflow with auditable checkpoints
- telemetry safety enforcement

### WP-C5: Production Readiness

Outputs:

- canary runbook and SLO thresholds
- rollback drills and incident playbook validation

## 6. Validation Plan

### Functional Validation

- explain output consistency with operator execution stages
- trace and metrics correctness under normal and error paths
- adapter operation semantics (idempotency, error mapping, version negotiation)
- security hardening behavior by role/classification/deletion state

### Integration Validation

- SQL path and adapter path produce consistent behavior for same workload
- observability records complete end-to-end journey for retrieval and assembly
- policy enforcement remains consistent under mixed client traffic

### Performance Validation

- observability overhead budget checks
- adapter overhead versus direct SQL baseline
- security policy overhead in high-QPS retrieval workloads

### Reliability Validation

- trace backpressure and retention behavior under spikes
- adapter retries and dedup behavior under transient failures
- purge workflow correctness under concurrent reads/writes

## 7. Rollout and Rollback

### Rollout Stages

1. internal canary with OX enabled, EI disabled.
2. controlled adapter pilot for selected tenants.
3. security hardening in advisory mode.
4. full enforce mode with agreed SLO guardrails.

### Rollback Rules

- disable adapter endpoints without changing SQL core contracts.
- disable extended trace capture while keeping minimal telemetry.
- revert security hardening to baseline mode if policy regressions appear.

## 8. Risks and Mitigations

1. Risk: high-cardinality telemetry causes cost and latency pressure.
   - Mitigation: sampling, item caps, tag allow-list, retention policies.
2. Risk: adapter contracts drift from SQL capability evolution.
   - Mitigation: compatibility matrix + mandatory conformance tests in release gate.
3. Risk: hardening changes break existing clients.
   - Mitigation: advisory-first rollout and compatibility warnings.
4. Risk: explain surfaces leak sensitive content.
   - Mitigation: default redaction and privilege-gated extended payload access.

## 9. Acceptance Gate for Milestone C

Milestone C is accepted only when all gates pass:

1. Production gate: observability surfaces and SLO monitors are live.
2. Integration gate: at least one adapter profile passes conformance end-to-end.
3. Security gate: hardening controls validated across SQL and adapter paths.
4. Operations gate: canary + rollback drills completed with no red risk.
