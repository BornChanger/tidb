# TiDB Agent Memory P0 Design: Context Assembly Capability

- Author(s): TBD
- Discussion PR: TBD
- Tracking Issue: TBD

## Overview

Agent quality depends on what context is injected into each model call. Most systems currently
assemble context in application code with inconsistent token accounting and weak observability.
This design adds a TiDB-side context assembly capability that builds ranked, token-bounded
context packs directly from memory retrieval results.

## Goals

- Produce deterministic context packs under a strict token budget.
- Support policy-based sectioning (system facts, user preferences, episodic evidence).
- Expose assembly rationale and dropped-candidate reasons for debugging.
- Minimize app-side logic by keeping assembly in SQL.

## Non-Goals

- Implement model-specific prompt templating in P0.
- Guarantee exact token parity for every provider tokenizer in P0.
- Replace orchestration frameworks (LangGraph, etc.) in P0.

## Detailed Design

### SQL Surface

P0 adds a table function:

```sql
SELECT *
FROM TIDB_AGENT_CONTEXT_ASSEMBLE(
  tenant_id         => 't-1',
  namespace         => 'assistant-default',
  query_text        => 'How do I rotate API keys?',
  query_vector      => '[0.21, ...]',
  token_budget      => 3000,
  reserve_output    => 800,
  max_items         => 30,
  section_policy    => '{"preferences":0.2,"facts":0.3,"episodes":0.5}',
  include_trace     => true
);
```

Returns:

- `assembled_context` (text)
- `assembly_manifest` (JSON)
- `estimated_input_tokens`
- `dropped_candidates` (JSON)

### Assembly Pipeline

Pipeline stages:

1. candidate retrieval (reuse hybrid retrieval operator)
2. section classification (`preference`, `fact`, `episode`, `procedure`)
3. token estimation
4. budgeted packing
5. final formatting and manifest emission

Pseudo logic:

```text
effective_budget = token_budget - reserve_output
for section in section_policy order:
  allocate section quota
  pick highest score items that fit remaining quota
fill remainder globally by score density (score / token_cost)
```

### Token Estimation

P0 supports two estimators:

- `approx_char_based`: deterministic and cheap, default.
- `provider_profile`: configurable multiplier for known model families.

System variable examples:

- `tidb_agent_context_token_estimator = 'approx_char_based'`
- `tidb_agent_context_token_margin_ratio = 0.12`

### Output Format

`assembled_context` uses fixed section delimiters:

```text
[SYSTEM_FACTS]
...
[USER_PREFERENCES]
...
[RECENT_EPISODES]
...
[PROCEDURES]
...
```

`assembly_manifest` contains:

- selected memory ids
- per-item score and estimated token cost
- quota usage by section
- dropped reason (`budget`, `policy`, `low_score`, `stale`)

### Planner and Execution

New logical operator: `LogicalAgentContextAssemble`.

- optimize candidate fetch and pack in one plan.
- push tenant/namespace/time filters down.
- compute token estimate in vectorized path.

New physical operator: `PhysicalAgentContextAssemble`.

- in-memory packer using quota + density policy.
- bounded memory footprint by `max_items` and `candidate_n`.

### Error Handling and Guards

- `effective_budget <= 0`: return explicit SQL error.
- malformed section policy JSON: return parse error.
- if no candidate selected, return empty context + manifest with reason.

### Compatibility

- parser: no new SQL grammar required (table function approach).
- planner/executor: additive operator, feature-gated.
- TiCDC/BR: no special behavior.

## Test Design

### Functional Tests

- deterministic assembly for fixed input and fixed candidates.
- token budget never exceeded under estimator + margin.
- section quota enforcement correctness.

### Scenario Tests

- low budget where only high-density memories fit.
- mixed short and long memories.
- empty retrieval result path.

### Compatibility Tests

- behavior with partitioned memory tables.
- behavior under restricted privilege users.

### Benchmark Tests

- assembly latency as function of candidate count.
- estimator overhead under high QPS.

## Impacts and Risks

Impacts:

- lower context-overflow failures.
- better operational visibility into why specific context is used.

Risks:

- approximation error in token estimation can under/over-fill context.
- deterministic rules can reduce diversity in marginal cases.

Mitigations:

- configurable token safety margin.
- optional random tie-breaker mode in later phase.

## Investigation and Alternatives

- Alternative A: assembly stays only in app layer.
  - Rejected: no shared SQL-level observability and repeated logic.
- Alternative B: exact provider tokenizer only.
  - Rejected for P0 due provider coupling and maintenance burden.

## Open Questions

- should future versions support model-specific formatting profiles?
- should context assembly and retrieval be merged into one statement type?
