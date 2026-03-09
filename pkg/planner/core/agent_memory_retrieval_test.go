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
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRankAgentMemoryCandidatesDeterministic(t *testing.T) {
	weights := agentMemoryRetrieveWeights{vectorWeight: 0.55, recencyWeight: 0.25, importanceWeight: 0.20}
	candidates := []agentMemoryRetrieveCandidate{
		{memoryID: 9, vectorScore: 0.8, recencyScore: 0.7, importanceScore: 0.6},
		{memoryID: 2, vectorScore: 0.9, recencyScore: 0.9, importanceScore: 0.9},
		{memoryID: 5, vectorScore: 0.8, recencyScore: 0.7, importanceScore: 0.6},
		{memoryID: 1, vectorScore: 0.8, recencyScore: 0.7, importanceScore: 0.6},
	}

	ranked, err := rankAgentMemoryCandidates(candidates, 4, weights)
	require.NoError(t, err)
	require.Len(t, ranked, 4)
	require.Equal(t, uint64(2), ranked[0].memoryID)
	require.Equal(t, uint64(1), ranked[1].memoryID)
	require.Equal(t, uint64(5), ranked[2].memoryID)
	require.Equal(t, uint64(9), ranked[3].memoryID)
}

func TestRankAgentMemoryCandidatesBoundedByTopK(t *testing.T) {
	weights := agentMemoryRetrieveWeights{vectorWeight: 0.55, recencyWeight: 0.25, importanceWeight: 0.20}
	candidates := []agentMemoryRetrieveCandidate{
		{memoryID: 1, vectorScore: 0.1, recencyScore: 0.1, importanceScore: 0.1},
		{memoryID: 2, vectorScore: 0.2, recencyScore: 0.2, importanceScore: 0.2},
		{memoryID: 3, vectorScore: 0.3, recencyScore: 0.3, importanceScore: 0.3},
	}

	ranked, err := rankAgentMemoryCandidates(candidates, 2, weights)
	require.NoError(t, err)
	require.Len(t, ranked, 2)
	require.Equal(t, uint64(3), ranked[0].memoryID)
	require.Equal(t, uint64(2), ranked[1].memoryID)
}

func TestRankAgentMemoryCandidatesWeightValidation(t *testing.T) {
	candidates := []agentMemoryRetrieveCandidate{{memoryID: 1, vectorScore: 0.5, recencyScore: 0.5, importanceScore: 0.5}}

	_, err := rankAgentMemoryCandidates(candidates, 1, agentMemoryRetrieveWeights{vectorWeight: 0.5, recencyWeight: 0.3, importanceWeight: 0.3})
	require.Error(t, err)

	_, err = rankAgentMemoryCandidates(candidates, 1, agentMemoryRetrieveWeights{vectorWeight: 0.65, recencyWeight: 0.25, importanceWeight: 0.20})
	require.Error(t, err)

	_, err = rankAgentMemoryCandidates(candidates, 1, agentMemoryRetrieveWeights{vectorWeight: 0.65, recencyWeight: 0.20, importanceWeight: 0.15})
	require.NoError(t, err)

	_, err = rankAgentMemoryCandidates(candidates, 0, agentMemoryRetrieveWeights{vectorWeight: 0.55, recencyWeight: 0.25, importanceWeight: 0.20})
	require.Error(t, err)
}

func TestRankAgentMemoryCandidatesNormalizeScores(t *testing.T) {
	weights := agentMemoryRetrieveWeights{vectorWeight: 0.55, recencyWeight: 0.25, importanceWeight: 0.20}
	candidates := []agentMemoryRetrieveCandidate{{
		memoryID:        7,
		vectorScore:     1.5,
		recencyScore:    -0.2,
		importanceScore: math.NaN(),
	}}

	ranked, err := rankAgentMemoryCandidates(candidates, 1, weights)
	require.NoError(t, err)
	require.Len(t, ranked, 1)
	require.Equal(t, 1.0, ranked[0].vectorScore)
	require.Equal(t, 0.0, ranked[0].recencyScore)
	require.Equal(t, 0.0, ranked[0].importanceScore)
	require.False(t, math.IsNaN(ranked[0].finalScore))
}
