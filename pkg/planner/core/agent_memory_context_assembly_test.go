// Copyright 2026 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAssembleAgentMemoryContextBudgetAndManifest(t *testing.T) {
	policy := agentMemoryContextAssemblyPolicy{
		totalBudgetTokens:       100,
		reservedOutputTokens:    20,
		budgetSafetyMarginRatio: 0.10,
		maxCandidateCount:       10,
	}
	candidates := []agentMemoryAssemblyCandidate{
		{memoryID: 1, section: "facts", payload: "alpha", estimatedTokens: 30, score: 0.95},
		{memoryID: 2, section: "facts", payload: "beta", estimatedTokens: 25, score: 0.90},
		{memoryID: 3, section: "reasoning", payload: "gamma", estimatedTokens: 20, score: 0.80},
	}

	result, err := assembleAgentMemoryContext(candidates, policy)
	require.NoError(t, err)
	require.Equal(t, 70, result.effectiveBudgetTokens)
	require.Equal(t, 55, result.usedTokens)
	require.Len(t, result.selected, 2)
	require.Contains(t, result.assembledContext, "alpha")
	require.Contains(t, result.assembledContext, "beta")
	require.NotContains(t, result.assembledContext, "gamma")

	manifest := make(map[uint64]agentMemoryContextAssemblyManifestItem)
	for _, item := range result.manifest {
		manifest[item.memoryID] = item
	}
	require.Equal(t, "included", manifest[1].decision)
	require.Equal(t, "included", manifest[2].decision)
	require.Equal(t, "dropped", manifest[3].decision)
	require.Equal(t, "budget_exceeded", manifest[3].reason)
}

func TestAssembleAgentMemoryContextSectionQuota(t *testing.T) {
	policy := agentMemoryContextAssemblyPolicy{
		totalBudgetTokens:       100,
		reservedOutputTokens:    0,
		budgetSafetyMarginRatio: 0,
		maxCandidateCount:       10,
		sectionPolicies: map[string]agentMemoryAssemblySectionPolicy{
			"facts": {maxItems: 1, maxTokens: 0},
		},
	}
	candidates := []agentMemoryAssemblyCandidate{
		{memoryID: 3, section: "facts", payload: "f-1", estimatedTokens: 10, score: 0.95},
		{memoryID: 1, section: "facts", payload: "f-2", estimatedTokens: 10, score: 0.90},
		{memoryID: 2, section: "reasoning", payload: "r-1", estimatedTokens: 10, score: 0.80},
	}

	result, err := assembleAgentMemoryContext(candidates, policy)
	require.NoError(t, err)
	require.Len(t, result.selected, 2)
	require.Equal(t, uint64(3), result.selected[0].memoryID)
	require.Equal(t, uint64(2), result.selected[1].memoryID)

	manifest := make(map[uint64]agentMemoryContextAssemblyManifestItem)
	for _, item := range result.manifest {
		manifest[item.memoryID] = item
	}
	require.Equal(t, "dropped", manifest[1].decision)
	require.Equal(t, "section_item_quota_exceeded", manifest[1].reason)
}

func TestAssembleAgentMemoryContextDeterministicPacking(t *testing.T) {
	policy := agentMemoryContextAssemblyPolicy{
		totalBudgetTokens:       100,
		reservedOutputTokens:    0,
		budgetSafetyMarginRatio: 0,
		maxCandidateCount:       10,
	}
	firstInput := []agentMemoryAssemblyCandidate{
		{memoryID: 9, section: "facts", payload: "p9", estimatedTokens: 10, score: 0.80},
		{memoryID: 2, section: "facts", payload: "p2", estimatedTokens: 10, score: 0.80},
		{memoryID: 5, section: "facts", payload: "p5", estimatedTokens: 10, score: 0.80},
	}
	secondInput := []agentMemoryAssemblyCandidate{
		{memoryID: 5, section: "facts", payload: "p5", estimatedTokens: 10, score: 0.80},
		{memoryID: 9, section: "facts", payload: "p9", estimatedTokens: 10, score: 0.80},
		{memoryID: 2, section: "facts", payload: "p2", estimatedTokens: 10, score: 0.80},
	}

	firstResult, err := assembleAgentMemoryContext(firstInput, policy)
	require.NoError(t, err)
	secondResult, err := assembleAgentMemoryContext(secondInput, policy)
	require.NoError(t, err)

	require.Equal(t, []uint64{2, 5, 9}, selectedMemoryIDs(firstResult.selected))
	require.Equal(t, []uint64{2, 5, 9}, selectedMemoryIDs(secondResult.selected))
	require.Equal(t, firstResult.assembledContext, secondResult.assembledContext)
}

func TestAssembleAgentMemoryContextFallbackTokenEstimateFromPayload(t *testing.T) {
	policy := agentMemoryContextAssemblyPolicy{
		totalBudgetTokens:       8,
		reservedOutputTokens:    0,
		budgetSafetyMarginRatio: 0,
		maxCandidateCount:       10,
	}
	candidates := []agentMemoryAssemblyCandidate{
		{memoryID: 1, section: "facts", payload: "abcdefghij", estimatedTokens: 0, score: 0.95},
		{memoryID: 2, section: "facts", payload: "manual", estimatedTokens: 5, score: 0.90},
	}

	result, err := assembleAgentMemoryContext(candidates, policy)
	require.NoError(t, err)
	require.Len(t, result.selected, 2)
	require.Equal(t, 8, result.usedTokens)
	require.Equal(t, 3, result.selected[0].estimatedTokens)

	manifest := make(map[uint64]agentMemoryContextAssemblyManifestItem)
	for _, item := range result.manifest {
		manifest[item.memoryID] = item
	}
	require.Equal(t, 3, manifest[1].estimatedTokens)
	require.Equal(t, "included", manifest[1].decision)
}

func TestAssembleAgentMemoryContextFallbackTokenEstimateProviderProfile(t *testing.T) {
	policy := agentMemoryContextAssemblyPolicy{
		totalBudgetTokens:                7,
		reservedOutputTokens:             0,
		budgetSafetyMarginRatio:          0,
		maxCandidateCount:                10,
		tokenEstimatorStrategy:           agentMemoryTokenEstimatorProviderProfile,
		tokenEstimatorProviderMultiplier: 2,
	}
	candidates := []agentMemoryAssemblyCandidate{
		{memoryID: 1, section: "facts", payload: "abcdefghij", estimatedTokens: 0, score: 0.95},
		{memoryID: 2, section: "facts", payload: "manual", estimatedTokens: 1, score: 0.90},
	}

	result, err := assembleAgentMemoryContext(candidates, policy)
	require.NoError(t, err)
	require.Len(t, result.selected, 2)
	require.Equal(t, 7, result.usedTokens)
	require.Equal(t, 6, result.selected[0].estimatedTokens)

	manifest := make(map[uint64]agentMemoryContextAssemblyManifestItem)
	for _, item := range result.manifest {
		manifest[item.memoryID] = item
	}
	require.Equal(t, 6, manifest[1].estimatedTokens)
	require.Equal(t, "included", manifest[1].decision)
}

func TestAssembleAgentMemoryContextNonEstimableCandidate(t *testing.T) {
	_, err := assembleAgentMemoryContext([]agentMemoryAssemblyCandidate{{
		memoryID:        1,
		section:         "facts",
		payload:         " \n\t ",
		estimatedTokens: 0,
		score:           0.1,
	}}, agentMemoryContextAssemblyPolicy{totalBudgetTokens: 100})
	require.ErrorContains(t, err, "estimated_tokens should be positive")
}

func TestAssembleAgentMemoryContextInvalidPolicy(t *testing.T) {
	_, err := assembleAgentMemoryContext(nil, agentMemoryContextAssemblyPolicy{totalBudgetTokens: 0})
	require.ErrorContains(t, err, "total_budget_tokens should be positive")

	_, err = assembleAgentMemoryContext(nil, agentMemoryContextAssemblyPolicy{totalBudgetTokens: 100, reservedOutputTokens: 100})
	require.ErrorContains(t, err, "reserved_output_tokens should be less than total_budget_tokens")

	_, err = assembleAgentMemoryContext(nil, agentMemoryContextAssemblyPolicy{totalBudgetTokens: 100, budgetSafetyMarginRatio: -0.1})
	require.ErrorContains(t, err, "budget_safety_margin_ratio should be in [0,1)")

	_, err = assembleAgentMemoryContext(nil, agentMemoryContextAssemblyPolicy{totalBudgetTokens: 100, budgetSafetyMarginRatio: 1.0})
	require.ErrorContains(t, err, "budget_safety_margin_ratio should be in [0,1)")

	_, err = assembleAgentMemoryContext(nil, agentMemoryContextAssemblyPolicy{
		totalBudgetTokens: 100,
		sectionPolicies: map[string]agentMemoryAssemblySectionPolicy{
			"facts": {maxItems: -1},
		},
	})
	require.ErrorContains(t, err, "section policy has negative limits")

	_, err = assembleAgentMemoryContext(nil, agentMemoryContextAssemblyPolicy{
		totalBudgetTokens:      100,
		tokenEstimatorStrategy: "unknown",
	})
	require.ErrorContains(t, err, "invalid token_estimator_strategy")

	_, err = assembleAgentMemoryContext(nil, agentMemoryContextAssemblyPolicy{
		totalBudgetTokens:                100,
		tokenEstimatorStrategy:           agentMemoryTokenEstimatorProviderProfile,
		tokenEstimatorProviderMultiplier: 0,
	})
	require.ErrorContains(t, err, "token_estimator_provider_multiplier should be positive")

	_, err = assembleAgentMemoryContext([]agentMemoryAssemblyCandidate{{memoryID: 1, estimatedTokens: 0, score: 0.1}}, agentMemoryContextAssemblyPolicy{totalBudgetTokens: 100})
	require.ErrorContains(t, err, "estimated_tokens should be positive")
}

func TestAssembleAgentMemoryContextCandidateCountBound(t *testing.T) {
	candidates := []agentMemoryAssemblyCandidate{
		{memoryID: 1, section: "facts", payload: "a", estimatedTokens: 10, score: 0.9},
		{memoryID: 2, section: "facts", payload: "b", estimatedTokens: 10, score: 0.8},
	}
	_, err := assembleAgentMemoryContext(candidates, agentMemoryContextAssemblyPolicy{totalBudgetTokens: 100, maxCandidateCount: 1})
	require.ErrorContains(t, err, "candidate_count exceeds limit")
}

func selectedMemoryIDs(selected []agentMemoryAssemblyCandidate) []uint64 {
	ids := make([]uint64, 0, len(selected))
	for _, item := range selected {
		ids = append(ids, item.memoryID)
	}
	return ids
}
