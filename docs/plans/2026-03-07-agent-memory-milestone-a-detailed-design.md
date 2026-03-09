# TiDB Agent Memory Milestone A Detailed Design (Foundation Baseline)

- Author(s): TBD
- Discussion PR: TBD
- Tracking Issue: TBD

## 1. Objective and Contract

Milestone A establishes the minimum foundation required for all later agent-memory tracks.
The contract is to deliver a stable schema profile and fail-closed tenant isolation baseline,
with backward-compatible rollout and rollback controls.

### In Scope

- Standardized Memory Schema baseline (`SS`)
- Multi-tenant Security baseline (`MT`)
- Cross-track contract definitions used by Milestones B/C

### Out of Scope

- Hybrid retrieval and context assembly execution operators
- Lifecycle policy scheduler and summarization jobs
- Explain/trace surfaces and external adapter sidecars

## 2. Entry and Exit Criteria

### Entry Criteria

1. Overview index and track docs are approved as roadmap baseline.
2. Existing memory tables can be read/written without new profile constraints.
3. Feature flags are available for agent-memory experimental path.

### Exit Criteria

1. Profile `v1` bootstrap can create canonical memory schema and compatibility views.
2. Tenant context is mandatory for agent-memory operators (fail-closed when absent).
3. Baseline audit trail exists for memory read/write/delete and policy decisions.
4. Upgrade/downgrade test path proves no irreversible compatibility break.

## 3. Deliverables

### D1: Schema Profile Baseline

- Canonical profile for episodic/semantic/procedural memory tables.
- Shared mandatory columns and value constraints.
- Compatibility views for stable query surface.
- Profile version registry table for migration control.

### D2: Security Baseline

- Mandatory tenant/namespace context variables.
- Policy enforcement hooks for memory data access path.
- Read-path and write-path fail-closed behavior.

### D3: Audit Baseline

- System audit table for memory operations.
- Minimal event taxonomy (`read`, `write`, `delete`, `policy_denied`).
- Low-cardinality metadata to support compliance evidence.

## 4. Detailed Design

### 4.1 Schema Profile (`SS`) Baseline

Canonical profile version: `agent_memory_profile_v1`.

Required logical entities:

1. episodic memories
2. semantic memories
3. procedural memories
4. profile version registry

Required profile invariants:

- stable primary key semantics across all memory classes
- normalized tenant identity fields (`tenant_id`, `namespace`, `subject_id`)
- confidence and importance values constrained to `[0,1]`
- state enum aligned with lifecycle track (`hot`, `warm`, `cold`, `archived`)

Compatibility view contract:

- `agent_memory_all` provides a union interface across all memory classes.
- `agent_memory_active` hides archived rows by default.
- `agent_memory_for_retrieval` provides retrieval-safe projection.

### 4.2 Tenant Security (`MT`) Baseline

Mandatory runtime context:

- `tidb_agent_tenant_id`
- `tidb_agent_namespace`

Enforcement rules:

1. if required context variables are absent, memory operators return explicit error.
2. planner and executor paths must not bypass tenant predicates.
3. policy checks are applied before retrieval and before context assembly.

Minimal policy shape:

- row predicate bound to tenant identity
- namespace allow-list predicate
- deny-by-default policy when context is invalid

### 4.3 Audit Baseline

Audit baseline records:

- actor identity
- operation type
- target object id/type
- timestamp
- policy decision (`allow`/`deny`)
- optional reason code

Design constraints:

- no raw sensitive payload in default audit rows
- bounded cardinality fields for stable storage/metrics cost

## 5. Dependency Ordering

Hard dependencies in Milestone A:

1. profile registry must be ready before migration tooling is enabled.
2. compatibility views must be ready before any application SQL migration.
3. tenant enforcement must be enabled before Milestone B retrieval operators.

Soft dependencies:

- richer classification and redaction policies can be deferred to Milestone C hardening.

## 6. Work Packages

### WP-A1: Profile Bootstrap and Version Registry

Outputs:

- bootstrap routine behavior spec
- profile version metadata spec
- rollback-safe migration sequencing

### WP-A2: Compatibility View Layer

Outputs:

- compatibility views and naming contract
- query compatibility rules for existing clients

### WP-A3: Tenant Context Enforcement

Outputs:

- required session variable contract
- fail-closed error taxonomy
- policy injection points in memory access path

### WP-A4: Baseline Auditing

Outputs:

- audit schema and event taxonomy
- retention and access policy defaults

## 7. Validation Plan

### Functional Validation

- bootstrap generates required schema objects and views.
- missing tenant context returns deny error.
- compatibility views return expected columns across memory classes.
- audit rows are emitted for success and deny cases.

### Compatibility Validation

- profile migration from baseline schema to v1 and back (where supported).
- mixed-version read compatibility through views.
- BR/TiCDC metadata consistency for profile and audit tables.

### Reliability Validation

- repeated bootstrap idempotency.
- policy checks under concurrent sessions.

## 8. Rollout and Rollback

### Rollout Stages

1. shadow mode: schema/profile objects created, enforcement disabled.
2. advisory mode: deny conditions produce warnings.
3. enforce mode: deny-by-default for missing/invalid tenant context.

### Rollback Rules

- enforcement flags can be disabled without dropping data objects.
- compatibility views remain available during rollback window.
- profile version metadata tracks rollback state explicitly.

## 9. Risks and Mitigations

1. Risk: existing clients omit tenant context and break at enforcement.
   - Mitigation: staged rollout with advisory mode and compatibility warnings.
2. Risk: schema rigidity blocks domain-specific extensions.
   - Mitigation: optional extension columns + profile evolution policy.
3. Risk: audit growth causes storage overhead.
   - Mitigation: retention defaults and compact event schema.

## 10. Acceptance Gate for Milestone A

Milestone A is accepted only when all gates pass:

1. Foundation gate: schema profile + version registry + compatibility views complete.
2. Security gate: tenant fail-closed behavior validated under functional tests.
3. Compatibility gate: upgrade/downgrade and backup/restore evidence captured.
4. Operational gate: audit baseline and retention controls verified.
