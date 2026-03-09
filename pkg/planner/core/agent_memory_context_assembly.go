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
	"fmt"
	"math"
	"sort"
	"strings"
)

type agentMemoryTokenEstimatorStrategy string

const (
	agentMemoryContextAssemblyDefaultMaxCandidateCount                                   = 10000
	agentMemoryTokenEstimatorApproxCharBased           agentMemoryTokenEstimatorStrategy = "approx_char_based"
	agentMemoryTokenEstimatorProviderProfile           agentMemoryTokenEstimatorStrategy = "provider_profile"
)

type agentMemoryAssemblyCandidate struct {
	memoryType      string
	memoryID        uint64
	section         string
	payload         string
	estimatedTokens int
	score           float64
}

type agentMemoryAssemblySectionPolicy struct {
	maxItems  int
	maxTokens int
}

type agentMemoryContextAssemblyPolicy struct {
	totalBudgetTokens                int
	reservedOutputTokens             int
	budgetSafetyMarginRatio          float64
	sectionPolicies                  map[string]agentMemoryAssemblySectionPolicy
	maxCandidateCount                int
	tokenEstimatorStrategy           agentMemoryTokenEstimatorStrategy
	tokenEstimatorProviderMultiplier float64
}

type agentMemoryContextAssemblyManifestItem struct {
	memoryType      string
	memoryID        uint64
	section         string
	estimatedTokens int
	rank            int
	decision        string
	reason          string
}

type agentMemoryContextAssemblyResult struct {
	effectiveBudgetTokens int
	usedTokens            int
	assembledContext      string
	selected              []agentMemoryAssemblyCandidate
	manifest              []agentMemoryContextAssemblyManifestItem
}

func assembleAgentMemoryContext(candidates []agentMemoryAssemblyCandidate, policy agentMemoryContextAssemblyPolicy) (agentMemoryContextAssemblyResult, error) {
	effectiveBudget, err := validateAgentMemoryContextAssemblyPolicy(policy)
	if err != nil {
		return agentMemoryContextAssemblyResult{}, err
	}
	tokenEstimatorStrategy, tokenEstimatorProviderMultiplier, err := normalizeAgentMemoryTokenEstimatorPolicy(policy)
	if err != nil {
		return agentMemoryContextAssemblyResult{}, err
	}
	maxCandidateCount := policy.maxCandidateCount
	if maxCandidateCount <= 0 {
		maxCandidateCount = agentMemoryContextAssemblyDefaultMaxCandidateCount
	}
	if len(candidates) > maxCandidateCount {
		return agentMemoryContextAssemblyResult{}, fmt.Errorf("candidate_count exceeds limit: %d > %d", len(candidates), maxCandidateCount)
	}

	normalizedCandidates := make([]agentMemoryAssemblyCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		estimatedTokens := candidate.estimatedTokens
		if estimatedTokens <= 0 {
			estimatedTokens, err = estimateAgentMemoryFallbackTokens(candidate, tokenEstimatorStrategy, tokenEstimatorProviderMultiplier)
			if err != nil {
				return agentMemoryContextAssemblyResult{}, err
			}
		}
		if estimatedTokens <= 0 {
			return agentMemoryContextAssemblyResult{}, fmt.Errorf("estimated_tokens should be positive for memory_id=%d", candidate.memoryID)
		}
		score := candidate.score
		if math.IsNaN(score) || math.IsInf(score, 0) {
			score = 0
		}
		normalizedCandidates = append(normalizedCandidates, agentMemoryAssemblyCandidate{
			memoryType:      candidate.memoryType,
			memoryID:        candidate.memoryID,
			section:         candidate.section,
			payload:         candidate.payload,
			estimatedTokens: estimatedTokens,
			score:           score,
		})
	}

	sort.SliceStable(normalizedCandidates, func(i, j int) bool {
		if normalizedCandidates[i].score != normalizedCandidates[j].score {
			return normalizedCandidates[i].score > normalizedCandidates[j].score
		}
		if normalizedCandidates[i].memoryType != normalizedCandidates[j].memoryType {
			return normalizedCandidates[i].memoryType < normalizedCandidates[j].memoryType
		}
		return normalizedCandidates[i].memoryID < normalizedCandidates[j].memoryID
	})

	selected := make([]agentMemoryAssemblyCandidate, 0, len(normalizedCandidates))
	manifest := make([]agentMemoryContextAssemblyManifestItem, 0, len(normalizedCandidates))
	usedTokens := 0
	sectionItems := make(map[string]int, len(policy.sectionPolicies))
	sectionTokens := make(map[string]int, len(policy.sectionPolicies))

	for idx, candidate := range normalizedCandidates {
		rank := idx + 1
		if quota, ok := policy.sectionPolicies[candidate.section]; ok {
			if quota.maxItems > 0 && sectionItems[candidate.section] >= quota.maxItems {
				manifest = append(manifest, agentMemoryContextAssemblyManifestItem{
					memoryType:      candidate.memoryType,
					memoryID:        candidate.memoryID,
					section:         candidate.section,
					estimatedTokens: candidate.estimatedTokens,
					rank:            rank,
					decision:        "dropped",
					reason:          "section_item_quota_exceeded",
				})
				continue
			}
			if quota.maxTokens > 0 && sectionTokens[candidate.section]+candidate.estimatedTokens > quota.maxTokens {
				manifest = append(manifest, agentMemoryContextAssemblyManifestItem{
					memoryType:      candidate.memoryType,
					memoryID:        candidate.memoryID,
					section:         candidate.section,
					estimatedTokens: candidate.estimatedTokens,
					rank:            rank,
					decision:        "dropped",
					reason:          "section_token_quota_exceeded",
				})
				continue
			}
		}
		if usedTokens+candidate.estimatedTokens > effectiveBudget {
			manifest = append(manifest, agentMemoryContextAssemblyManifestItem{
				memoryType:      candidate.memoryType,
				memoryID:        candidate.memoryID,
				section:         candidate.section,
				estimatedTokens: candidate.estimatedTokens,
				rank:            rank,
				decision:        "dropped",
				reason:          "budget_exceeded",
			})
			continue
		}

		selected = append(selected, candidate)
		usedTokens += candidate.estimatedTokens
		sectionItems[candidate.section]++
		sectionTokens[candidate.section] += candidate.estimatedTokens
		manifest = append(manifest, agentMemoryContextAssemblyManifestItem{
			memoryType:      candidate.memoryType,
			memoryID:        candidate.memoryID,
			section:         candidate.section,
			estimatedTokens: candidate.estimatedTokens,
			rank:            rank,
			decision:        "included",
			reason:          "",
		})
	}

	if usedTokens > effectiveBudget {
		return agentMemoryContextAssemblyResult{}, fmt.Errorf("internal invariant violated: used_tokens(%d) > effective_budget_tokens(%d)", usedTokens, effectiveBudget)
	}

	var builder strings.Builder
	for i, item := range selected {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString("[")
		builder.WriteString(item.section)
		builder.WriteString("] ")
		builder.WriteString(item.payload)
	}

	return agentMemoryContextAssemblyResult{
		effectiveBudgetTokens: effectiveBudget,
		usedTokens:            usedTokens,
		assembledContext:      builder.String(),
		selected:              selected,
		manifest:              manifest,
	}, nil
}

func validateAgentMemoryContextAssemblyPolicy(policy agentMemoryContextAssemblyPolicy) (int, error) {
	if policy.totalBudgetTokens <= 0 {
		return 0, fmt.Errorf("total_budget_tokens should be positive")
	}
	if policy.reservedOutputTokens < 0 {
		return 0, fmt.Errorf("reserved_output_tokens should be non-negative")
	}
	if policy.reservedOutputTokens >= policy.totalBudgetTokens {
		return 0, fmt.Errorf("reserved_output_tokens should be less than total_budget_tokens")
	}
	if policy.budgetSafetyMarginRatio < 0 || policy.budgetSafetyMarginRatio >= 1 {
		return 0, fmt.Errorf("budget_safety_margin_ratio should be in [0,1)")
	}
	for section, quota := range policy.sectionPolicies {
		if quota.maxItems < 0 || quota.maxTokens < 0 {
			return 0, fmt.Errorf("section policy has negative limits for section=%s", section)
		}
	}
	marginTokens := int(math.Ceil(float64(policy.totalBudgetTokens) * policy.budgetSafetyMarginRatio))
	effectiveBudget := policy.totalBudgetTokens - policy.reservedOutputTokens - marginTokens
	if effectiveBudget <= 0 {
		return 0, fmt.Errorf("effective_budget_tokens should be positive")
	}
	return effectiveBudget, nil
}

func normalizeAgentMemoryTokenEstimatorPolicy(policy agentMemoryContextAssemblyPolicy) (agentMemoryTokenEstimatorStrategy, float64, error) {
	strategy := agentMemoryTokenEstimatorStrategy(strings.TrimSpace(string(policy.tokenEstimatorStrategy)))
	if strategy == "" {
		return agentMemoryTokenEstimatorApproxCharBased, 1, nil
	}

	switch strategy {
	case agentMemoryTokenEstimatorApproxCharBased:
		return strategy, 1, nil
	case agentMemoryTokenEstimatorProviderProfile:
		multiplier := policy.tokenEstimatorProviderMultiplier
		if multiplier <= 0 || math.IsNaN(multiplier) || math.IsInf(multiplier, 0) {
			return "", 0, fmt.Errorf("token_estimator_provider_multiplier should be positive")
		}
		return strategy, multiplier, nil
	default:
		return "", 0, fmt.Errorf("invalid token_estimator_strategy: %s", strategy)
	}
}

func estimateAgentMemoryFallbackTokens(candidate agentMemoryAssemblyCandidate, strategy agentMemoryTokenEstimatorStrategy, providerMultiplier float64) (int, error) {
	payloadLength := len(strings.TrimSpace(candidate.payload))
	if payloadLength <= 0 {
		return 0, fmt.Errorf("estimated_tokens should be positive for memory_id=%d", candidate.memoryID)
	}

	baseEstimate := float64(payloadLength) / 4.0
	charBasedEstimate := math.Ceil(baseEstimate)
	switch strategy {
	case agentMemoryTokenEstimatorApproxCharBased:
		return int(charBasedEstimate), nil
	case agentMemoryTokenEstimatorProviderProfile:
		return int(math.Ceil(charBasedEstimate * providerMultiplier)), nil
	default:
		return 0, fmt.Errorf("invalid token_estimator_strategy: %s", strategy)
	}
}
