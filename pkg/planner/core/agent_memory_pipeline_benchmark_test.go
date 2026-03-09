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
	"testing"
)

var (
	agentMemoryPipelineBenchmarkSinkResult agentMemoryPipelineResult
	agentMemoryObserveBenchmarkSinkResult  agentMemoryPipelineResult
)

type agentMemoryBenchmarkObservationSink struct{}

func (agentMemoryBenchmarkObservationSink) recordTrace(agentMemoryTraceRecord) {}

func (agentMemoryBenchmarkObservationSink) recordMetrics(agentMemoryObservationMetrics) {}

func BenchmarkRunAgentMemoryPipeline(b *testing.B) {
	for _, n := range []int{32, 128, 512} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			candidates := benchmarkAgentMemoryCandidates(n)
			config := benchmarkAgentMemoryPipelineConfig(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := runAgentMemoryPipeline(candidates, config)
				if err != nil {
					b.Fatal(err)
				}
				agentMemoryPipelineBenchmarkSinkResult = result
			}
		})
	}
}

func BenchmarkObserveAgentMemoryPipeline(b *testing.B) {
	for _, n := range []int{32, 128, 512} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			candidates := benchmarkAgentMemoryCandidates(n)
			config := benchmarkAgentMemoryPipelineConfig(n)
			obsConfig := agentMemoryObservabilityConfig{
				traceEnable:         true,
				traceSampleRatio:    0,
				traceMaxItems:       64,
				traceCapturePayload: false,
			}
			sink := agentMemoryBenchmarkObservationSink{}
			randFn := func() float64 {
				return 1
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := observeAgentMemoryPipeline(candidates, config, obsConfig, sink, randFn)
				if err != nil {
					b.Fatal(err)
				}
				agentMemoryObserveBenchmarkSinkResult = result
			}
		})
	}
}

func benchmarkAgentMemoryPipelineConfig(candidateN int) agentMemoryPipelineConfig {
	limitK := 32
	if candidateN < limitK {
		limitK = candidateN
	}
	return agentMemoryPipelineConfig{
		includeArchived: false,
		retrieveLimitK:  limitK,
		retrieveWeights: agentMemoryRetrieveWeights{
			vectorWeight:     0.55,
			recencyWeight:    0.25,
			importanceWeight: 0.20,
		},
		assemblyPolicy: agentMemoryContextAssemblyPolicy{
			totalBudgetTokens:       4096,
			reservedOutputTokens:    512,
			budgetSafetyMarginRatio: 0.05,
			maxCandidateCount:       candidateN,
		},
	}
}

func benchmarkAgentMemoryCandidates(n int) []agentMemoryPipelineCandidate {
	candidates := make([]agentMemoryPipelineCandidate, 0, n)
	for i := 0; i < n; i++ {
		state := agentMemoryPipelineStateHot
		switch i % 4 {
		case 1:
			state = agentMemoryPipelineStateWarm
		case 2:
			state = agentMemoryPipelineStateCold
		case 3:
			state = agentMemoryPipelineStateArchive
		}
		memoryType := "episodic"
		switch i % 3 {
		case 1:
			memoryType = "semantic"
		case 2:
			memoryType = "procedural"
		}
		vectorScore := float64((i%97)+1) / 100
		recencyScore := float64((i%89)+1) / 100
		importanceScore := float64((i%79)+1) / 100
		candidates = append(candidates, agentMemoryPipelineCandidate{
			memoryType:      memoryType,
			memoryID:        uint64(i + 1),
			section:         "facts",
			payload:         "{}",
			lifecycleState:  state,
			estimatedTokens: 16 + (i % 48),
			vectorScore:     vectorScore,
			recencyScore:    recencyScore,
			importanceScore: importanceScore,
		})
	}
	return candidates
}
