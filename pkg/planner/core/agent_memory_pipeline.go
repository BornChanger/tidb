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
	"strconv"
	"strings"

	"github.com/pingcap/tidb/pkg/sessionctx/vardef"
	"github.com/pingcap/tidb/pkg/sessionctx/variable"
	"github.com/pingcap/tidb/pkg/util/dbterror/plannererrors"
)

type agentMemoryPipelineState string

const (
	agentMemoryPipelineStateHot     agentMemoryPipelineState = "hot"
	agentMemoryPipelineStateWarm    agentMemoryPipelineState = "warm"
	agentMemoryPipelineStateCold    agentMemoryPipelineState = "cold"
	agentMemoryPipelineStateArchive agentMemoryPipelineState = "archived"
)

type agentMemoryPipelineCandidate struct {
	memoryType      string
	memoryID        uint64
	section         string
	payload         string
	lifecycleState  agentMemoryPipelineState
	estimatedTokens int
	vectorScore     float64
	recencyScore    float64
	importanceScore float64
}

type agentMemoryPipelineConfig struct {
	includeArchived bool
	retrieveLimitK  int
	retrieveWeights agentMemoryRetrieveWeights
	assemblyPolicy  agentMemoryContextAssemblyPolicy
	sessionVars     *variable.SessionVars
}

type agentMemoryPipelineRetrieved struct {
	memoryType      string
	memoryID        uint64
	section         string
	payload         string
	lifecycleState  agentMemoryPipelineState
	estimatedTokens int
	vectorScore     float64
	recencyScore    float64
	importanceScore float64
	finalScore      float64
}

type agentMemoryPipelineResult struct {
	retrieved []agentMemoryPipelineRetrieved
	assembly  agentMemoryContextAssemblyResult
}

type agentMemoryPipelineIdentity struct {
	memoryType string
	memoryID   uint64
}

func runAgentMemoryPipeline(candidates []agentMemoryPipelineCandidate, config agentMemoryPipelineConfig) (agentMemoryPipelineResult, error) {
	if config.sessionVars != nil {
		tenantID, _ := config.sessionVars.GetSystemVar(vardef.TiDBAgentTenantID)
		namespace, _ := config.sessionVars.GetSystemVar(vardef.TiDBAgentNamespace)
		if tenantID == "" || namespace == "" {
			return agentMemoryPipelineResult{}, plannererrors.ErrSpecificAccessDenied.GenWithStackByArgs("AGENT_MEMORY_TENANT_CONTEXT")
		}
	}

	eligible := make([]agentMemoryPipelineCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !config.includeArchived && candidate.lifecycleState == agentMemoryPipelineStateArchive {
			continue
		}
		eligible = append(eligible, candidate)
	}

	retrieveCandidates := make([]agentMemoryRetrieveCandidate, 0, len(eligible))
	candidateByID := make(map[agentMemoryPipelineIdentity]agentMemoryPipelineCandidate, len(eligible))
	for _, candidate := range eligible {
		retrieveCandidates = append(retrieveCandidates, agentMemoryRetrieveCandidate{
			memoryType:      candidate.memoryType,
			memoryID:        candidate.memoryID,
			vectorScore:     candidate.vectorScore,
			recencyScore:    candidate.recencyScore,
			importanceScore: candidate.importanceScore,
		})
		candidateByID[agentMemoryPipelineIdentity{memoryType: candidate.memoryType, memoryID: candidate.memoryID}] = candidate
	}

	ranked, err := rankAgentMemoryCandidates(retrieveCandidates, config.retrieveLimitK, config.retrieveWeights)
	if err != nil {
		return agentMemoryPipelineResult{}, err
	}

	retrieved := make([]agentMemoryPipelineRetrieved, 0, len(ranked))
	assemblyInput := make([]agentMemoryAssemblyCandidate, 0, len(ranked))
	for _, rankedCandidate := range ranked {
		base := candidateByID[agentMemoryPipelineIdentity{memoryType: rankedCandidate.memoryType, memoryID: rankedCandidate.memoryID}]
		retrieved = append(retrieved, agentMemoryPipelineRetrieved{
			memoryType:      rankedCandidate.memoryType,
			memoryID:        rankedCandidate.memoryID,
			section:         base.section,
			payload:         base.payload,
			lifecycleState:  base.lifecycleState,
			estimatedTokens: base.estimatedTokens,
			vectorScore:     rankedCandidate.vectorScore,
			recencyScore:    rankedCandidate.recencyScore,
			importanceScore: rankedCandidate.importanceScore,
			finalScore:      rankedCandidate.finalScore,
		})
		assemblyInput = append(assemblyInput, agentMemoryAssemblyCandidate{
			memoryType:      rankedCandidate.memoryType,
			memoryID:        rankedCandidate.memoryID,
			section:         base.section,
			payload:         base.payload,
			estimatedTokens: base.estimatedTokens,
			score:           rankedCandidate.finalScore,
		})
	}

	assemblyPolicy := applyAgentMemoryTokenEstimatorSessionVars(config.assemblyPolicy, config.sessionVars)
	assemblyResult, err := assembleAgentMemoryContext(assemblyInput, assemblyPolicy)
	if err != nil {
		return agentMemoryPipelineResult{}, err
	}

	return agentMemoryPipelineResult{
		retrieved: retrieved,
		assembly:  assemblyResult,
	}, nil
}

func applyAgentMemoryTokenEstimatorSessionVars(policy agentMemoryContextAssemblyPolicy, sessionVars *variable.SessionVars) agentMemoryContextAssemblyPolicy {
	if sessionVars == nil {
		return policy
	}

	if strategy, ok := sessionVars.GetSystemVar(vardef.TiDBAgentMemoryContextAssemblyTokenEstimatorStrategy); ok {
		policy.tokenEstimatorStrategy = agentMemoryTokenEstimatorStrategy(strings.TrimSpace(strategy))
	}

	if multiplierVal, ok := sessionVars.GetSystemVar(vardef.TiDBAgentMemoryContextAssemblyTokenEstimatorProviderMultiplier); ok {
		if multiplier, err := strconv.ParseFloat(strings.TrimSpace(multiplierVal), 64); err == nil {
			policy.tokenEstimatorProviderMultiplier = multiplier
		}
	}

	if totalBudgetVal, ok := sessionVars.GetSystemVar(vardef.TiDBAgentMemoryContextAssemblyTotalBudgetTokens); ok {
		if totalBudget, err := strconv.Atoi(strings.TrimSpace(totalBudgetVal)); err == nil {
			policy.totalBudgetTokens = totalBudget
		}
	}

	if reservedOutputVal, ok := sessionVars.GetSystemVar(vardef.TiDBAgentMemoryContextAssemblyReservedOutputTokens); ok {
		if reservedOutput, err := strconv.Atoi(strings.TrimSpace(reservedOutputVal)); err == nil {
			policy.reservedOutputTokens = reservedOutput
		}
	}

	if budgetMarginVal, ok := sessionVars.GetSystemVar(vardef.TiDBAgentMemoryContextAssemblyBudgetSafetyMarginRatio); ok {
		if budgetMargin, err := strconv.ParseFloat(strings.TrimSpace(budgetMarginVal), 64); err == nil {
			policy.budgetSafetyMarginRatio = budgetMargin
		}
	}

	return policy
}
