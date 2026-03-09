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

	"github.com/pingcap/tidb/pkg/sessionctx/vardef"
	"github.com/pingcap/tidb/pkg/sessionctx/variable"
	"github.com/stretchr/testify/require"
)

func TestRunAgentMemoryPipelineDeterministicOutput(t *testing.T) {
	config := agentMemoryPipelineConfig{
		includeArchived: false,
		retrieveLimitK:  3,
		retrieveWeights: agentMemoryRetrieveWeights{vectorWeight: 0.55, recencyWeight: 0.25, importanceWeight: 0.20},
		assemblyPolicy: agentMemoryContextAssemblyPolicy{
			totalBudgetTokens:       120,
			reservedOutputTokens:    20,
			budgetSafetyMarginRatio: 0,
			maxCandidateCount:       10,
		},
	}
	firstInput := []agentMemoryPipelineCandidate{
		{memoryType: "episodic", memoryID: 9, section: "facts", payload: "f9", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 20, vectorScore: 0.8, recencyScore: 0.8, importanceScore: 0.8},
		{memoryType: "episodic", memoryID: 2, section: "facts", payload: "f2", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 20, vectorScore: 0.8, recencyScore: 0.8, importanceScore: 0.8},
		{memoryType: "episodic", memoryID: 5, section: "facts", payload: "f5", lifecycleState: agentMemoryPipelineStateCold, estimatedTokens: 20, vectorScore: 0.8, recencyScore: 0.8, importanceScore: 0.8},
	}
	secondInput := []agentMemoryPipelineCandidate{
		{memoryType: "episodic", memoryID: 5, section: "facts", payload: "f5", lifecycleState: agentMemoryPipelineStateCold, estimatedTokens: 20, vectorScore: 0.8, recencyScore: 0.8, importanceScore: 0.8},
		{memoryType: "episodic", memoryID: 9, section: "facts", payload: "f9", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 20, vectorScore: 0.8, recencyScore: 0.8, importanceScore: 0.8},
		{memoryType: "episodic", memoryID: 2, section: "facts", payload: "f2", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 20, vectorScore: 0.8, recencyScore: 0.8, importanceScore: 0.8},
	}

	firstResult, err := runAgentMemoryPipeline(firstInput, config)
	require.NoError(t, err)
	secondResult, err := runAgentMemoryPipeline(secondInput, config)
	require.NoError(t, err)
	require.Equal(t, []uint64{2, 5, 9}, selectedPipelineMemoryIDs(firstResult.retrieved))
	require.Equal(t, []uint64{2, 5, 9}, selectedPipelineMemoryIDs(secondResult.retrieved))
	require.Equal(t, firstResult.assembly.assembledContext, secondResult.assembly.assembledContext)
}

func TestRunAgentMemoryPipelineLifecycleStateInfluence(t *testing.T) {
	config := agentMemoryPipelineConfig{
		includeArchived: false,
		retrieveLimitK:  2,
		retrieveWeights: agentMemoryRetrieveWeights{vectorWeight: 0.55, recencyWeight: 0.25, importanceWeight: 0.20},
		assemblyPolicy: agentMemoryContextAssemblyPolicy{
			totalBudgetTokens:       80,
			reservedOutputTokens:    0,
			budgetSafetyMarginRatio: 0,
			maxCandidateCount:       10,
		},
	}
	candidates := []agentMemoryPipelineCandidate{
		{memoryType: "episodic", memoryID: 1, section: "facts", payload: "hot", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 20, vectorScore: 0.7, recencyScore: 0.7, importanceScore: 0.7},
		{memoryType: "episodic", memoryID: 2, section: "facts", payload: "archived", lifecycleState: agentMemoryPipelineStateArchive, estimatedTokens: 20, vectorScore: 0.99, recencyScore: 0.99, importanceScore: 0.99},
		{memoryType: "episodic", memoryID: 3, section: "facts", payload: "cold", lifecycleState: agentMemoryPipelineStateCold, estimatedTokens: 20, vectorScore: 0.8, recencyScore: 0.8, importanceScore: 0.8},
	}

	result, err := runAgentMemoryPipeline(candidates, config)
	require.NoError(t, err)
	require.Equal(t, []uint64{3, 1}, selectedPipelineMemoryIDs(result.retrieved))

	config.includeArchived = true
	resultWithArchived, err := runAgentMemoryPipeline(candidates, config)
	require.NoError(t, err)
	require.Equal(t, []uint64{2, 3}, selectedPipelineMemoryIDs(resultWithArchived.retrieved))
}

func TestRunAgentMemoryPipelineRepresentativeWorkloads(t *testing.T) {
	type manifestExpectation struct {
		decision string
		reason   string
	}

	tests := []struct {
		name                  string
		config                agentMemoryPipelineConfig
		candidates            []agentMemoryPipelineCandidate
		expectedRetrievedIDs  []uint64
		expectedSelectedIDs   []uint64
		expectedManifestOrder []uint64
		expectedManifest      map[uint64]manifestExpectation
		expectedContext       string
	}{
		{
			name: "exclude archived and enforce section plus budget drops",
			config: agentMemoryPipelineConfig{
				includeArchived: false,
				retrieveLimitK:  4,
				retrieveWeights: agentMemoryRetrieveWeights{vectorWeight: 1, recencyWeight: 0, importanceWeight: 0},
				assemblyPolicy: agentMemoryContextAssemblyPolicy{
					totalBudgetTokens:       25,
					reservedOutputTokens:    0,
					budgetSafetyMarginRatio: 0,
					sectionPolicies: map[string]agentMemoryAssemblySectionPolicy{
						"facts": {maxItems: 1},
					},
					maxCandidateCount: 10,
				},
			},
			candidates: []agentMemoryPipelineCandidate{
				{memoryType: "episodic", memoryID: 101, section: "facts", payload: "fact-hot", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 8, vectorScore: 0.90, recencyScore: 0.90, importanceScore: 0.90},
				{memoryType: "episodic", memoryID: 102, section: "tasks", payload: "task-warm", lifecycleState: agentMemoryPipelineStateWarm, estimatedTokens: 10, vectorScore: 0.80, recencyScore: 0.80, importanceScore: 0.80},
				{memoryType: "episodic", memoryID: 103, section: "facts", payload: "fact-archived", lifecycleState: agentMemoryPipelineStateArchive, estimatedTokens: 6, vectorScore: 0.99, recencyScore: 0.99, importanceScore: 0.99},
				{memoryType: "episodic", memoryID: 104, section: "notes", payload: "note-cold", lifecycleState: agentMemoryPipelineStateCold, estimatedTokens: 12, vectorScore: 0.70, recencyScore: 0.70, importanceScore: 0.70},
				{memoryType: "episodic", memoryID: 105, section: "facts", payload: "fact-cold", lifecycleState: agentMemoryPipelineStateCold, estimatedTokens: 7, vectorScore: 0.75, recencyScore: 0.75, importanceScore: 0.75},
			},
			expectedRetrievedIDs:  []uint64{101, 102, 105, 104},
			expectedSelectedIDs:   []uint64{101, 102},
			expectedManifestOrder: []uint64{101, 102, 105, 104},
			expectedManifest: map[uint64]manifestExpectation{
				101: {decision: "included", reason: ""},
				102: {decision: "included", reason: ""},
				105: {decision: "dropped", reason: "section_item_quota_exceeded"},
				104: {decision: "dropped", reason: "budget_exceeded"},
			},
			expectedContext: "[facts] fact-hot\n[tasks] task-warm",
		},
		{
			name: "include archived and enforce section token quota",
			config: agentMemoryPipelineConfig{
				includeArchived: true,
				retrieveLimitK:  3,
				retrieveWeights: agentMemoryRetrieveWeights{vectorWeight: 0.55, recencyWeight: 0.25, importanceWeight: 0.20},
				assemblyPolicy: agentMemoryContextAssemblyPolicy{
					totalBudgetTokens:       20,
					reservedOutputTokens:    0,
					budgetSafetyMarginRatio: 0,
					sectionPolicies: map[string]agentMemoryAssemblySectionPolicy{
						"tasks": {maxTokens: 6},
					},
					maxCandidateCount: 10,
				},
			},
			candidates: []agentMemoryPipelineCandidate{
				{memoryType: "episodic", memoryID: 201, section: "facts", payload: "archived-fact", lifecycleState: agentMemoryPipelineStateArchive, estimatedTokens: 10, vectorScore: 0.95, recencyScore: 0.95, importanceScore: 0.95},
				{memoryType: "episodic", memoryID: 202, section: "tasks", payload: "hot-task", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 6, vectorScore: 0.92, recencyScore: 0.70, importanceScore: 0.60},
				{memoryType: "episodic", memoryID: 203, section: "tasks", payload: "warm-task", lifecycleState: agentMemoryPipelineStateWarm, estimatedTokens: 7, vectorScore: 0.88, recencyScore: 0.85, importanceScore: 0.80},
				{memoryType: "episodic", memoryID: 204, section: "notes", payload: "cold-note", lifecycleState: agentMemoryPipelineStateCold, estimatedTokens: 9, vectorScore: 0.75, recencyScore: 0.90, importanceScore: 0.90},
				{memoryType: "episodic", memoryID: 205, section: "facts", payload: "hot-fact", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 8, vectorScore: 0.89, recencyScore: 0.40, importanceScore: 0.30},
			},
			expectedRetrievedIDs:  []uint64{201, 203, 204},
			expectedSelectedIDs:   []uint64{201, 204},
			expectedManifestOrder: []uint64{201, 203, 204},
			expectedManifest: map[uint64]manifestExpectation{
				201: {decision: "included", reason: ""},
				203: {decision: "dropped", reason: "section_token_quota_exceeded"},
				204: {decision: "included", reason: ""},
			},
			expectedContext: "[facts] archived-fact\n[notes] cold-note",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := runAgentMemoryPipeline(test.candidates, test.config)
			require.NoError(t, err)

			require.Equal(t, test.expectedRetrievedIDs, selectedPipelineMemoryIDs(result.retrieved))
			require.Equal(t, test.expectedSelectedIDs, selectedAssemblyMemoryIDs(result.assembly.selected))
			require.Equal(t, test.expectedManifestOrder, manifestPipelineMemoryIDs(result.assembly.manifest))
			require.Equal(t, test.expectedContext, result.assembly.assembledContext)

			require.Len(t, result.assembly.manifest, len(test.expectedManifest))
			for _, item := range result.assembly.manifest {
				expected, ok := test.expectedManifest[item.memoryID]
				require.Truef(t, ok, "unexpected manifest entry for memory_id=%d", item.memoryID)
				require.Equal(t, expected.decision, item.decision)
				require.Equal(t, expected.reason, item.reason)
			}
		})
	}
}

func TestRunAgentMemoryPipelineCompositeIdentity(t *testing.T) {
	config := agentMemoryPipelineConfig{
		includeArchived: true,
		retrieveLimitK:  2,
		retrieveWeights: agentMemoryRetrieveWeights{vectorWeight: 0.55, recencyWeight: 0.25, importanceWeight: 0.20},
		assemblyPolicy:  agentMemoryContextAssemblyPolicy{totalBudgetTokens: 80},
	}
	candidates := []agentMemoryPipelineCandidate{
		{memoryType: "episodic", memoryID: 7, section: "facts", payload: "episodic", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 20, vectorScore: 0.9, recencyScore: 0.9, importanceScore: 0.9},
		{memoryType: "semantic", memoryID: 7, section: "facts", payload: "semantic", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 20, vectorScore: 0.8, recencyScore: 0.8, importanceScore: 0.8},
	}

	result, err := runAgentMemoryPipeline(candidates, config)
	require.NoError(t, err)
	require.Len(t, result.retrieved, 2)
	require.Equal(t, "episodic", result.retrieved[0].payload)
	require.Equal(t, "semantic", result.retrieved[1].payload)
	require.Equal(t, "episodic", result.assembly.selected[0].payload)
	require.Equal(t, "semantic", result.assembly.selected[1].payload)
}

func TestRunAgentMemoryPipelineTokenEstimatorSessionVars(t *testing.T) {
	sessionVars := variable.NewSessionVars(nil)
	sessionVars.GlobalVarsAccessor = variable.NewMockGlobalAccessor4Tests()
	err := sessionVars.SetSystemVar(vardef.TiDBAgentTenantID, "tenant_test")
	require.NoError(t, err)
	err = sessionVars.SetSystemVar(vardef.TiDBAgentNamespace, "ns_test")
	require.NoError(t, err)

	err = sessionVars.SetSystemVar(vardef.TiDBAgentMemoryContextAssemblyTokenEstimatorStrategy, "provider_profile")
	require.NoError(t, err)
	err = sessionVars.SetSystemVar(vardef.TiDBAgentMemoryContextAssemblyTokenEstimatorProviderMultiplier, "2")
	require.NoError(t, err)

	config := agentMemoryPipelineConfig{
		includeArchived: false,
		retrieveLimitK:  2,
		retrieveWeights: agentMemoryRetrieveWeights{vectorWeight: 0.55, recencyWeight: 0.25, importanceWeight: 0.20},
		sessionVars:     sessionVars,
		assemblyPolicy: agentMemoryContextAssemblyPolicy{
			totalBudgetTokens:       7,
			reservedOutputTokens:    0,
			budgetSafetyMarginRatio: 0,
			maxCandidateCount:       10,
		},
	}
	candidates := []agentMemoryPipelineCandidate{
		{memoryType: "episodic", memoryID: 1, section: "facts", payload: "abcdefghij", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 0, vectorScore: 0.95, recencyScore: 0.95, importanceScore: 0.95},
		{memoryType: "episodic", memoryID: 2, section: "facts", payload: "manual", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 1, vectorScore: 0.90, recencyScore: 0.90, importanceScore: 0.90},
	}

	result, err := runAgentMemoryPipeline(candidates, config)
	require.NoError(t, err)
	require.Len(t, result.assembly.selected, 2)
	require.Equal(t, 7, result.assembly.usedTokens)
	require.Equal(t, 6, result.assembly.selected[0].estimatedTokens)
}

func TestRunAgentMemoryPipelineTenantContextFailClosed(t *testing.T) {
	sessionVars := variable.NewSessionVars(nil)
	sessionVars.GlobalVarsAccessor = variable.NewMockGlobalAccessor4Tests()

	config := agentMemoryPipelineConfig{
		includeArchived: false,
		retrieveLimitK:  1,
		retrieveWeights: agentMemoryRetrieveWeights{vectorWeight: 1},
		sessionVars:     sessionVars,
		assemblyPolicy: agentMemoryContextAssemblyPolicy{
			totalBudgetTokens:       32,
			reservedOutputTokens:    0,
			budgetSafetyMarginRatio: 0,
			maxCandidateCount:       10,
		},
	}
	candidates := []agentMemoryPipelineCandidate{
		{memoryType: "episodic", memoryID: 1, section: "facts", payload: "ok", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 2, vectorScore: 0.9, recencyScore: 0.9, importanceScore: 0.9},
	}

	_, err := runAgentMemoryPipeline(candidates, config)
	require.ErrorContains(t, err, "AGENT_MEMORY_TENANT_CONTEXT")

	err = sessionVars.SetSystemVar(vardef.TiDBAgentTenantID, "tenant_test")
	require.NoError(t, err)
	err = sessionVars.SetSystemVar(vardef.TiDBAgentNamespace, "ns_test")
	require.NoError(t, err)

	result, err := runAgentMemoryPipeline(candidates, config)
	require.NoError(t, err)
	require.Equal(t, []uint64{1}, selectedPipelineMemoryIDs(result.retrieved))
}

func TestRunAgentMemoryPipelineContextAssemblyBudgetSessionVarsRejectInvalidEffectiveBudget(t *testing.T) {
	sessionVars := variable.NewSessionVars(nil)
	sessionVars.GlobalVarsAccessor = variable.NewMockGlobalAccessor4Tests()
	err := sessionVars.SetSystemVar(vardef.TiDBAgentTenantID, "tenant_test")
	require.NoError(t, err)
	err = sessionVars.SetSystemVar(vardef.TiDBAgentNamespace, "ns_test")
	require.NoError(t, err)

	err = sessionVars.SetSystemVar(vardef.TiDBAgentMemoryContextAssemblyReservedOutputTokens, "1")
	require.NoError(t, err)
	err = sessionVars.SetSystemVar(vardef.TiDBAgentMemoryContextAssemblyTotalBudgetTokens, "10")
	require.NoError(t, err)
	err = sessionVars.SetSystemVar(vardef.TiDBAgentMemoryContextAssemblyReservedOutputTokens, "9")
	require.NoError(t, err)
	err = sessionVars.SetSystemVar(vardef.TiDBAgentMemoryContextAssemblyBudgetSafetyMarginRatio, "0.2")
	require.NoError(t, err)

	config := agentMemoryPipelineConfig{
		includeArchived: false,
		retrieveLimitK:  1,
		retrieveWeights: agentMemoryRetrieveWeights{vectorWeight: 1},
		sessionVars:     sessionVars,
		assemblyPolicy: agentMemoryContextAssemblyPolicy{
			totalBudgetTokens:       64,
			reservedOutputTokens:    0,
			budgetSafetyMarginRatio: 0,
			maxCandidateCount:       10,
		},
	}
	candidates := []agentMemoryPipelineCandidate{
		{memoryType: "episodic", memoryID: 1, section: "facts", payload: "ok", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 1, vectorScore: 0.9, recencyScore: 0.9, importanceScore: 0.9},
	}

	_, err = runAgentMemoryPipeline(candidates, config)
	require.ErrorContains(t, err, "effective_budget_tokens should be positive")
}

func TestRunAgentMemoryPipelineTokenEstimatorFallback(t *testing.T) {
	config := agentMemoryPipelineConfig{
		includeArchived: false,
		retrieveLimitK:  3,
		retrieveWeights: agentMemoryRetrieveWeights{vectorWeight: 0.55, recencyWeight: 0.25, importanceWeight: 0.20},
		assemblyPolicy: agentMemoryContextAssemblyPolicy{
			totalBudgetTokens:       9,
			reservedOutputTokens:    0,
			budgetSafetyMarginRatio: 0,
			maxCandidateCount:       10,
		},
	}
	candidates := []agentMemoryPipelineCandidate{
		{memoryType: "episodic", memoryID: 1, section: "facts", payload: "  abcdefghij  ", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 0, vectorScore: 0.95, recencyScore: 0.95, importanceScore: 0.95},
		{memoryType: "episodic", memoryID: 2, section: "facts", payload: "wxyz", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: -3, vectorScore: 0.90, recencyScore: 0.90, importanceScore: 0.90},
		{memoryType: "episodic", memoryID: 3, section: "facts", payload: "manual", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 5, vectorScore: 0.85, recencyScore: 0.85, importanceScore: 0.85},
	}

	result, err := runAgentMemoryPipeline(candidates, config)
	require.NoError(t, err)
	require.Equal(t, []uint64{1, 2, 3}, selectedPipelineMemoryIDs(result.retrieved))
	require.Len(t, result.assembly.selected, 3)
	require.Equal(t, 9, result.assembly.usedTokens)
	require.Equal(t, uint64(1), result.assembly.selected[0].memoryID)
	require.Equal(t, uint64(2), result.assembly.selected[1].memoryID)
	require.Equal(t, uint64(3), result.assembly.selected[2].memoryID)
	require.Equal(t, 3, result.assembly.selected[0].estimatedTokens)
	require.Equal(t, 1, result.assembly.selected[1].estimatedTokens)
	require.Equal(t, 5, result.assembly.selected[2].estimatedTokens)

	manifest := make(map[uint64]agentMemoryContextAssemblyManifestItem)
	for _, item := range result.assembly.manifest {
		manifest[item.memoryID] = item
	}
	require.Equal(t, 3, manifest[1].estimatedTokens)
	require.Equal(t, 1, manifest[2].estimatedTokens)
	require.Equal(t, "included", manifest[1].decision)
	require.Equal(t, "included", manifest[2].decision)
}

func selectedPipelineMemoryIDs(retrieved []agentMemoryPipelineRetrieved) []uint64 {
	ids := make([]uint64, 0, len(retrieved))
	for _, item := range retrieved {
		ids = append(ids, item.memoryID)
	}
	return ids
}

func selectedAssemblyMemoryIDs(selected []agentMemoryAssemblyCandidate) []uint64 {
	ids := make([]uint64, 0, len(selected))
	for _, item := range selected {
		ids = append(ids, item.memoryID)
	}
	return ids
}

func manifestPipelineMemoryIDs(manifest []agentMemoryContextAssemblyManifestItem) []uint64 {
	ids := make([]uint64, 0, len(manifest))
	for _, item := range manifest {
		ids = append(ids, item.memoryID)
	}
	return ids
}
