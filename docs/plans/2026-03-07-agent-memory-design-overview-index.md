# TiDB Agent Memory Design: Overview Index

- Author(s): TBD
- Discussion PR: TBD
- Tracking Issue: TBD

## Purpose

This document is the master index for the TiDB Agent Memory design package created on 2026-03-07.
It links all design tracks, defines dependency order, and provides a practical rollout sequence
for implementation planning and issue breakdown.

## Scope Summary

The package covers 7 design tracks:

- P0 capability tracks (core retrieval path):
  - Hybrid Retrieval Operator
  - Memory Lifecycle Engine
  - Context Assembly Capability
- P1 capability tracks (standardization and production hardening):
  - Standardized Memory Schema
  - Observability and Explainability
  - Multi-tenant Security and Governance
  - Ecosystem Integrations

## Document Catalog

| Track | Phase | Primary Outcome | Design Doc |
|---|---|---|---|
| Hybrid Retrieval Operator | P0 | Optimizer-aware fused ranking (vector + recency + importance + filters) | `docs/plans/2026-03-07-agent-memory-hybrid-retrieval-design.md` |
| Memory Lifecycle Engine | P0 | Hot/warm/cold/archive transitions with policy-driven jobs | `docs/plans/2026-03-07-agent-memory-lifecycle-engine-design.md` |
| Context Assembly Capability | P0 | Deterministic token-budgeted context packs | `docs/plans/2026-03-07-agent-memory-context-assembly-design.md` |
| Standardized Memory Schema | P1 | Canonical episodic/semantic/procedural schema profile | `docs/plans/2026-03-07-agent-memory-standardized-schema-design.md` |
| Observability and Explainability | P1 | Retrieval/assembly traceability and score decomposition | `docs/plans/2026-03-07-agent-memory-observability-explainability-design.md` |
| Multi-tenant Security and Governance | P1 | Tenant isolation, redaction, audit, and deletion controls | `docs/plans/2026-03-07-agent-memory-multi-tenant-security-design.md` |
| Ecosystem Integrations | P1 | mem0/LangGraph/MCP integration contracts and adapters | `docs/plans/2026-03-07-agent-memory-ecosystem-integrations-design.md` |

## Capability Comparison (External Landscape)

Snapshot: 2026-03 (conservative scoring based on publicly documented capabilities).

Legend:

- `I`: implemented as product-level native capability.
- `P`: partial; usually requires app/framework-side orchestration.
- `M`: missing or not native in core product.
- `Plan-P0`: planned in this package as P0 delivery scope.
- `Plan-P1`: planned in this package as P1 delivery scope.

Capability columns map to this design package tracks:

- `HR`: Hybrid Retrieval
- `LC`: Lifecycle/TTL and tiering
- `CA`: Context Assembly
- `SS`: Standardized Memory Schema
- `OX`: Observability/Explainability for memory path
- `MT`: Multi-tenant Security
- `EI`: Ecosystem Integrations

| Platform | HR | LC | CA | SS | OX | MT | EI | Notes |
|---|---|---|---|---|---|---|---|---|
| mem0 | P | P | P | P | P | P | I | Strong framework-layer integration; many controls are integration-time decisions. |
| LangGraph | P | P | P | M | P | P | I | Provides long-term memory primitives and semantic search, but schema and assembly policy are app-defined. |
| mnemos (qiffang) | I | P | P | P | M | P | I | Implements hybrid retrieval and multi-agent plugins; lifecycle, schema governance, and explainability are mostly service-level or partial. |
| Pinecone | I | P | M | M | P | I | I | Strong hybrid retrieval + namespace isolation; no native end-to-end context assembly contract. |
| Weaviate | I | I | M | M | P | I | I | Hybrid retrieval, explain score, multi-tenancy and TTL/tenant-state operations are available. |
| Elasticsearch (Elastic) | I | I | M | M | P | I | P | Strong retrieval fusion, ILM, and DLS/FLS; agent-memory assembly and schema are mostly app conventions. |
| MongoDB Atlas | I | I | M | M | P | I | I | Vector + hybrid search + TTL + integrations are strong; memory-specific assembly/schema are app-side. |
| TiDB (Current Baseline) | P | P | M | M | P | I | P | Already has relevant primitives (SQL + vector index foundation + TTL patterns), but no unified agent-memory product surface yet. |
| TiDB (Planned in This Package) | Plan-P0 | Plan-P0 | Plan-P0 | Plan-P1 | Plan-P1 | Plan-P1 | Plan-P1 | Direct mapping to 7 design docs in this package; target is an integrated memory data plane + governance stack. |

TiDB feature mapping behind the planned row:

- `HR` -> `2026-03-07-agent-memory-hybrid-retrieval-design.md` (P0)
- `LC` -> `2026-03-07-agent-memory-lifecycle-engine-design.md` (P0)
- `CA` -> `2026-03-07-agent-memory-context-assembly-design.md` (P0)
- `SS` -> `2026-03-07-agent-memory-standardized-schema-design.md` (P1)
- `OX` -> `2026-03-07-agent-memory-observability-explainability-design.md` (P1)
- `MT` -> `2026-03-07-agent-memory-multi-tenant-security-design.md` (P1)
- `EI` -> `2026-03-07-agent-memory-ecosystem-integrations-design.md` (P1)

Reference entry points:

- LangGraph memory and semantic search:
  - `https://blog.langchain.dev/launching-long-term-memory-support-in-langgraph/`
  - `https://blog.langchain.dev/semantic-search-for-langgraph-memory`
- mem0 project overview:
  - `https://github.com/mem0ai/mem0`
- mnemos (official repo and design):
  - `https://github.com/qiffang/mnemos`
  - `https://github.com/qiffang/mnemos/blob/main/docs/DESIGN.md`
  - `https://github.com/qiffang/mnemos/blob/main/server/schema.sql`
- Pinecone hybrid search and multitenancy:
  - `https://docs.pinecone.io/guides/search/hybrid-search`
  - `https://docs.pinecone.io/guides/index-data/implement-multitenancy`
- Weaviate hybrid and multi-tenancy operations:
  - `https://weaviate.io/developers/weaviate/search/hybrid`
  - `https://weaviate.io/developers/weaviate/manage-collections/multi-tenancy`
- Elastic RRF, ILM, and field/document-level security:
  - `https://www.elastic.co/docs/reference/elasticsearch/rest-apis/reciprocal-rank-fusion`
  - `https://www.elastic.co/docs/manage-data/lifecycle/index-lifecycle-management`
  - `https://www.elastic.co/docs/deploy-manage/users-roles/cluster-or-deployment-auth/controlling-access-at-document-field-level`
- MongoDB Vector Search and TTL:
  - `https://www.mongodb.com/docs/atlas/atlas-vector-search/vector-search-overview/`
  - `https://www.mongodb.com/docs/manual/core/index-ttl/`

## Dependency Graph

Logical dependency recommendations:

1. `Standardized Memory Schema` is foundational for stable interfaces.
2. `Multi-tenant Security and Governance` should be established early as a hard constraint.
3. `Hybrid Retrieval Operator` depends on schema and security guarantees.
4. `Context Assembly Capability` depends on retrieval outputs and schema conventions.
5. `Memory Lifecycle Engine` depends on schema and should integrate with retrieval filters.
6. `Observability and Explainability` spans retrieval + assembly + lifecycle.
7. `Ecosystem Integrations` depends on stabilized operation contracts from all tracks above.

Compact view:

```text
Standardized Schema -----> Hybrid Retrieval -----> Context Assembly
       |                         |                        |
       |                         +----------+             |
       v                                    v             v
Multi-tenant Security ---------> Memory Lifecycle -----> Observability
                                                      \
                                                       +-> Ecosystem Integrations
```

## Recommended Rollout Plan

### Milestone A: Foundation Baseline

Target tracks:

- Standardized Memory Schema
- Multi-tenant Security and Governance (minimum isolation baseline)

Exit criteria:

- canonical tables/views available in one reference environment
- tenant/namespace enforcement active for memory operators
- compatibility checks pass for BR/TiCDC-sensitive metadata handling

### Milestone B: Core Retrieval Path (P0)

Target tracks:

- Hybrid Retrieval Operator
- Context Assembly Capability
- Memory Lifecycle Engine (core transitions)

Exit criteria:

- deterministic top-k retrieval with configurable score weights
- token-budgeted context pack output with explainable manifests
- background lifecycle jobs running with pause/resume/cancel controls

### Milestone C: Production Hardening (P1)

Target tracks:

- Observability and Explainability
- Ecosystem Integrations
- Security hardening extensions (redaction/audit/deletion workflows)

Exit criteria:

- explain surfaces and trace tables available
- at least one reference adapter profile validated end-to-end
- security/audit controls verified under multi-tenant traffic

## Milestone Detailed Design Package

The following docs execute the roadmap path above as milestone-level detailed design contracts:

- Milestone A (Foundation Baseline):
  - `docs/plans/2026-03-07-agent-memory-milestone-a-detailed-design.md`
- Milestone B (Core Retrieval Path):
  - `docs/plans/2026-03-07-agent-memory-milestone-b-detailed-design.md`
- Milestone C (Production Hardening):
  - `docs/plans/2026-03-07-agent-memory-milestone-c-detailed-design.md`

## Milestone Issue Backlog Package

- Executable issue backlog (epics + sub-issues + acceptance criteria):
  - `docs/plans/2026-03-07-agent-memory-milestone-issue-backlog.md`

## User Spec Package

- Client-facing integration user spec (Claude Code / OpenCode / OpenClaw):
  - `docs/plans/2026-03-07-agent-memory-user-spec-claude-opencode-openclaw.md`

## Cross-track Contract Checklist

To avoid drift, these contracts should be shared across all tracks:

- **Identity contract**: `tenant_id`, `namespace`, `subject_id` semantics are consistent.
- **Scoring contract**: score component naming (`vector_score`, `recency_score`, etc.) remains stable.
- **Lifecycle contract**: state values (`hot/warm/cold/archived`) are canonical.
- **Trace contract**: trace ids and operation names are consistent across operators/adapters.
- **Versioning contract**: schema/profile version is discoverable and backward-compatible where possible.

## Suggested Issue Breakdown Template

For each track, create one umbrella issue plus sub-issues:

1. Parser/planner/executor (if applicable)
2. DDL/system table changes (if applicable)
3. Metrics/trace surfaces
4. Tests (unit + integration)
5. Docs and examples

Suggested labels:

- `type/enhancement`
- `component/planner`, `component/executor`, `component/session`, `component/docs` (as needed)

## Reading Order by Audience

- **Infra/DB kernel engineers**: schema -> retrieval -> lifecycle -> observability
- **Agent platform engineers**: retrieval -> context assembly -> integrations -> security
- **Security/compliance reviewers**: security -> observability -> lifecycle -> schema
- **PM/roadmap owners**: this overview -> milestone sections -> per-track risks

## Risk Overview

Top cross-track risks:

1. Contract drift between SQL operators and adapters.
2. Security policies not enforced uniformly in all access paths.
3. Cost/latency regressions from unbounded candidate pools and traces.
4. Quality regressions from lifecycle summarization or dedup thresholds.

Mitigation baseline:

- conformance tests for shared contracts
- strict default limits for candidate and trace cardinality
- staged rollout with shadow/validation mode where practical

## Status Tracking

Use this index as a living tracker by appending:

- implementation PR links per track
- benchmark result links
- rollout readiness checklists per milestone

---

### Quick Links

- `docs/plans/2026-03-07-agent-memory-hybrid-retrieval-design.md`
- `docs/plans/2026-03-07-agent-memory-lifecycle-engine-design.md`
- `docs/plans/2026-03-07-agent-memory-context-assembly-design.md`
- `docs/plans/2026-03-07-agent-memory-standardized-schema-design.md`
- `docs/plans/2026-03-07-agent-memory-observability-explainability-design.md`
- `docs/plans/2026-03-07-agent-memory-multi-tenant-security-design.md`
- `docs/plans/2026-03-07-agent-memory-ecosystem-integrations-design.md`
- `docs/plans/2026-03-07-agent-memory-milestone-a-detailed-design.md`
- `docs/plans/2026-03-07-agent-memory-milestone-b-detailed-design.md`
- `docs/plans/2026-03-07-agent-memory-milestone-c-detailed-design.md`
- `docs/plans/2026-03-07-agent-memory-milestone-issue-backlog.md`
- `docs/plans/2026-03-07-agent-memory-user-spec-claude-opencode-openclaw.md`
