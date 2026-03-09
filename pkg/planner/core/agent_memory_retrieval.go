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
)

const agentMemoryWeightTolerance = 1e-6

const (
	agentMemoryDefaultVectorWeight     = 0.55
	agentMemoryDefaultRecencyWeight    = 0.25
	agentMemoryDefaultImportanceWeight = 0.20
)

type agentMemoryRetrieveWeights struct {
	vectorWeight     float64
	recencyWeight    float64
	importanceWeight float64
}

type agentMemoryRetrieveCandidate struct {
	memoryType      string
	memoryID        uint64
	vectorScore     float64
	recencyScore    float64
	importanceScore float64
}

type agentMemoryScoredCandidate struct {
	memoryType      string
	memoryID        uint64
	vectorScore     float64
	recencyScore    float64
	importanceScore float64
	finalScore      float64
}

func validateAgentMemoryRetrieveWeights(weights agentMemoryRetrieveWeights) error {
	for name, value := range map[string]float64{
		"vector_weight":     weights.vectorWeight,
		"recency_weight":    weights.recencyWeight,
		"importance_weight": weights.importanceWeight,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
			return fmt.Errorf("invalid %s: %v, expected value in [0,1]", name, value)
		}
	}
	sum := weights.vectorWeight + weights.recencyWeight + weights.importanceWeight
	if math.Abs(sum-1) > agentMemoryWeightTolerance {
		return fmt.Errorf("invalid weight sum %v, expected 1.0", sum)
	}
	return nil
}

func normalizeAgentMemoryScore(score float64) float64 {
	if math.IsNaN(score) || math.IsInf(score, 0) {
		return 0
	}
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func rankAgentMemoryCandidates(candidates []agentMemoryRetrieveCandidate, limitK int, weights agentMemoryRetrieveWeights) ([]agentMemoryScoredCandidate, error) {
	if limitK <= 0 {
		return nil, fmt.Errorf("invalid limit_k: %d", limitK)
	}
	if err := validateAgentMemoryRetrieveWeights(weights); err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	ranked := make([]agentMemoryScoredCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		vectorScore := normalizeAgentMemoryScore(candidate.vectorScore)
		recencyScore := normalizeAgentMemoryScore(candidate.recencyScore)
		importanceScore := normalizeAgentMemoryScore(candidate.importanceScore)
		finalScore := weights.vectorWeight*vectorScore + weights.recencyWeight*recencyScore + weights.importanceWeight*importanceScore
		ranked = append(ranked, agentMemoryScoredCandidate{
			memoryType:      candidate.memoryType,
			memoryID:        candidate.memoryID,
			vectorScore:     vectorScore,
			recencyScore:    recencyScore,
			importanceScore: importanceScore,
			finalScore:      finalScore,
		})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].finalScore != ranked[j].finalScore {
			return ranked[i].finalScore > ranked[j].finalScore
		}
		if ranked[i].recencyScore != ranked[j].recencyScore {
			return ranked[i].recencyScore > ranked[j].recencyScore
		}
		if ranked[i].importanceScore != ranked[j].importanceScore {
			return ranked[i].importanceScore > ranked[j].importanceScore
		}
		if ranked[i].vectorScore != ranked[j].vectorScore {
			return ranked[i].vectorScore > ranked[j].vectorScore
		}
		if ranked[i].memoryType != ranked[j].memoryType {
			return ranked[i].memoryType < ranked[j].memoryType
		}
		return ranked[i].memoryID < ranked[j].memoryID
	})

	if len(ranked) > limitK {
		ranked = ranked[:limitK]
	}
	return ranked, nil
}
