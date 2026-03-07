# TiDB Agent Memory P1 Design: Multi-tenant Security and Governance

- Author(s): TBD
- Discussion PR: TBD
- Tracking Issue: TBD

## Overview

Agent memory contains user preferences, history, and potentially sensitive personal or enterprise
data. This design adds strong multi-tenant isolation, policy enforcement, and auditable access
controls for TiDB-based memory deployments.

## Goals

- Enforce tenant isolation for reads, writes, and retrieval operations.
- Provide practical controls for PII minimization and redaction.
- Add auditable access trails for compliance requirements.
- Prevent cross-tenant leakage through optimizer or cache behavior.

## Non-Goals

- Replace enterprise IAM platforms.
- Guarantee full legal compliance for every jurisdiction without deployment policy.

## Threat Model

Primary risks:

1. accidental cross-tenant retrieval due to missing predicates.
2. privileged misuse of raw memory payload.
3. data remanence after deletion requests.
4. telemetry leakage of sensitive values.

## Detailed Design

### Isolation Model

Every memory row is bound to `tenant_id` and `namespace`.

P1 introduces mandatory tenant context variables for memory operators:

- `tidb_agent_tenant_id`
- `tidb_agent_namespace`

If absent, memory operators fail closed with explicit error.

### Row-level Policy Enforcement

New policy object:

```sql
CREATE AGENT MEMORY POLICY tenant_policy_a
  ON app_memory.agent_memory_all
  REQUIRE tenant_id = @@tidb_agent_tenant_id
  REQUIRE namespace IN JSON_ARRAY_ELEMENTS(@@tidb_agent_namespace_allowlist);
```

Policy is automatically injected for:

- retrieval operators
- context assembly operators
- trace table read APIs

### Data Classification and Redaction

Payload classification levels:

- `public`, `internal`, `restricted`, `highly_sensitive`

Redaction policy:

- default query role sees only redacted payload for `restricted+`.
- privileged role can request unredacted payload with explicit audit reason.

Masking is implemented via policy-aware projection operators.

### Encryption and Key Handling

- rely on TiKV encryption-at-rest for base data.
- optional column-level payload encryption for high sensitivity.
- key rotation process documented for memory tables.

### Audit and Deletion Trail

New audit table: `mysql.tidb_agent_memory_audit`

- `audit_id`, `tenant_id`, `actor`, `action`, `object_type`, `object_id`, `reason`, `event_time`

Deletion support:

- `ADMIN AGENT MEMORY PURGE` records tombstone + hard-delete status.
- purge workflow includes embedding/index artifact deletion checkpoints.

### Cache and Plan Safety

- include tenant namespace in cache key for memory operator plans.
- disallow sharing operator-level cached candidates across tenants.

### Telemetry Safety

- traces and logs store ids/scores by default, no raw payload.
- optional payload capture requires elevated privilege and explicit opt-in.

### Compatibility

- additive security policy layer; existing table ACL remains valid.
- retrieval/context features require tenant context and policy compatibility.

## Test Design

### Functional Tests

- missing tenant context fails closed.
- row-level policy blocks cross-tenant reads.
- redaction rules by role and classification.

### Scenario Tests

- concurrent tenants with same subject ids.
- deletion request while retrieval traffic is active.

### Compatibility Tests

- policy interactions with views and partitioned tables.
- backup/restore of policy metadata and audit trails.

### Benchmark Tests

- overhead of policy enforcement on retrieval latency.
- audit write amplification under high QPS.

## Impacts and Risks

Impacts:

- materially stronger security posture for enterprise memory deployments.
- clearer compliance story for audits and deletion workflows.

Risks:

- stricter defaults can break existing non-compliant clients.
- redaction logic can increase CPU and complexity.

Mitigations:

- staged rollout with compatibility mode and warnings.
- policy linter before enforcement in production.

## Investigation and Alternatives

- Alternative A: enforce isolation only in application.
  - Rejected: too error-prone and difficult to audit.
- Alternative B: separate cluster per tenant only.
  - Rejected: expensive and operationally heavy for many workloads.

## Open Questions

- should policy language reuse existing TiDB policy grammar or introduce dedicated syntax?
- should high-sensitivity payload default to detached blob storage in future phase?
