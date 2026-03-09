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

func TestObserveAgentMemoryPipelineNormalPath(t *testing.T) {
	sink := &agentMemoryObservationSinkMock{}
	pipelineConfig := agentMemoryPipelineConfig{
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
	obsConfig := agentMemoryObservabilityConfig{
		traceEnable:          true,
		traceSampleRatio:     1,
		traceMaxItems:        1,
		traceCapturePayload:  false,
		metricSampleDisabled: false,
	}
	candidates := []agentMemoryPipelineCandidate{
		{memoryType: "episodic", memoryID: 1, section: "facts", payload: "payload-a", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 20, vectorScore: 0.9, recencyScore: 0.8, importanceScore: 0.7},
		{memoryType: "semantic", memoryID: 2, section: "facts", payload: "payload-b", lifecycleState: agentMemoryPipelineStateWarm, estimatedTokens: 20, vectorScore: 0.8, recencyScore: 0.7, importanceScore: 0.9},
	}

	result, err := observeAgentMemoryPipeline(candidates, pipelineConfig, obsConfig, sink, func() float64 { return 0 })
	require.NoError(t, err)
	require.Len(t, result.retrieved, 2)
	require.Len(t, sink.metrics, 1)
	require.Equal(t, 2, sink.metrics[0].candidateCount)
	require.Equal(t, 2, sink.metrics[0].selectedCount)
	require.Equal(t, "ok", sink.metrics[0].status)

	require.Len(t, sink.traces, 1)
	trace := sink.traces[0]
	require.Equal(t, 2, trace.candidateCount)
	require.Equal(t, 2, trace.selectedCount)
	require.Len(t, trace.items, 1)
	require.Empty(t, trace.items[0].payload)
}

func TestObserveAgentMemoryPipelineErrorPathAlwaysSampled(t *testing.T) {
	sink := &agentMemoryObservationSinkMock{}
	pipelineConfig := agentMemoryPipelineConfig{
		includeArchived: false,
		retrieveLimitK:  0,
		retrieveWeights: agentMemoryRetrieveWeights{vectorWeight: 0.55, recencyWeight: 0.25, importanceWeight: 0.20},
		assemblyPolicy:  agentMemoryContextAssemblyPolicy{totalBudgetTokens: 100},
	}
	obsConfig := agentMemoryObservabilityConfig{traceEnable: true, traceSampleRatio: 0, traceMaxItems: 10}
	_, err := observeAgentMemoryPipeline([]agentMemoryPipelineCandidate{{memoryType: "episodic", memoryID: 1, section: "facts", payload: "payload", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 20, vectorScore: 0.7, recencyScore: 0.7, importanceScore: 0.7}}, pipelineConfig, obsConfig, sink, func() float64 { return 0.99 })
	require.Error(t, err)
	require.Len(t, sink.metrics, 1)
	require.Equal(t, "error", sink.metrics[0].status)
	require.Len(t, sink.traces, 1)
	require.NotEmpty(t, sink.traces[0].errorCode)
}

func TestObserveAgentMemoryPipelineSuccessSamplingControl(t *testing.T) {
	sink := &agentMemoryObservationSinkMock{}
	pipelineConfig := agentMemoryPipelineConfig{
		includeArchived: false,
		retrieveLimitK:  1,
		retrieveWeights: agentMemoryRetrieveWeights{vectorWeight: 0.55, recencyWeight: 0.25, importanceWeight: 0.20},
		assemblyPolicy:  agentMemoryContextAssemblyPolicy{totalBudgetTokens: 60},
	}
	obsConfig := agentMemoryObservabilityConfig{traceEnable: true, traceSampleRatio: 0, traceMaxItems: 10}
	_, err := observeAgentMemoryPipeline([]agentMemoryPipelineCandidate{{memoryType: "episodic", memoryID: 1, section: "facts", payload: "payload", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 20, vectorScore: 0.7, recencyScore: 0.7, importanceScore: 0.7}}, pipelineConfig, obsConfig, sink, func() float64 { return 0.5 })
	require.NoError(t, err)
	require.Len(t, sink.metrics, 1)
	require.Len(t, sink.traces, 0)
}

func TestObserveAgentMemoryPipelinePayloadCaptureOptIn(t *testing.T) {
	sink := &agentMemoryObservationSinkMock{}
	pipelineConfig := agentMemoryPipelineConfig{
		includeArchived: false,
		retrieveLimitK:  1,
		retrieveWeights: agentMemoryRetrieveWeights{vectorWeight: 0.55, recencyWeight: 0.25, importanceWeight: 0.20},
		assemblyPolicy:  agentMemoryContextAssemblyPolicy{totalBudgetTokens: 60},
	}
	obsConfig := agentMemoryObservabilityConfig{traceEnable: true, traceSampleRatio: 1, traceMaxItems: 10, traceCapturePayload: true}
	_, err := observeAgentMemoryPipeline([]agentMemoryPipelineCandidate{{memoryType: "episodic", memoryID: 1, section: "facts", payload: "sensitive-payload", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 20, vectorScore: 0.7, recencyScore: 0.7, importanceScore: 0.7}}, pipelineConfig, obsConfig, sink, func() float64 { return 0 })
	require.NoError(t, err)
	require.Len(t, sink.traces, 1)
	require.Equal(t, "sensitive-payload", sink.traces[0].items[0].payload)
}

func TestObserveAgentMemoryPipelinePayloadCaptureUsesCompositeIdentity(t *testing.T) {
	sink := &agentMemoryObservationSinkMock{}
	pipelineConfig := agentMemoryPipelineConfig{
		includeArchived: false,
		retrieveLimitK:  2,
		retrieveWeights: agentMemoryRetrieveWeights{vectorWeight: 0.55, recencyWeight: 0.25, importanceWeight: 0.20},
		assemblyPolicy:  agentMemoryContextAssemblyPolicy{totalBudgetTokens: 80},
	}
	obsConfig := agentMemoryObservabilityConfig{traceEnable: true, traceSampleRatio: 1, traceMaxItems: 10, traceCapturePayload: true}
	_, err := observeAgentMemoryPipeline([]agentMemoryPipelineCandidate{
		{memoryType: "episodic", memoryID: 7, section: "facts", payload: "episodic-payload", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 20, vectorScore: 0.9, recencyScore: 0.9, importanceScore: 0.9},
		{memoryType: "semantic", memoryID: 7, section: "facts", payload: "semantic-payload", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 20, vectorScore: 0.8, recencyScore: 0.8, importanceScore: 0.8},
	}, pipelineConfig, obsConfig, sink, func() float64 { return 0 })
	require.NoError(t, err)
	require.Len(t, sink.traces, 1)

	payloads := map[string]string{}
	for _, item := range sink.traces[0].items {
		payloads[item.memoryType] = item.payload
	}
	require.Equal(t, "episodic-payload", payloads["episodic"])
	require.Equal(t, "semantic-payload", payloads["semantic"])
}

func TestObserveAgentMemoryPipelineObservabilityOverheadGuard(t *testing.T) {
	sink := &agentMemoryObservationSinkMock{}
	pipelineConfig := agentMemoryPipelineConfig{
		includeArchived: false,
		retrieveLimitK:  2,
		retrieveWeights: agentMemoryRetrieveWeights{vectorWeight: 0.55, recencyWeight: 0.25, importanceWeight: 0.20},
		assemblyPolicy: agentMemoryContextAssemblyPolicy{
			totalBudgetTokens:       100,
			reservedOutputTokens:    0,
			budgetSafetyMarginRatio: 0,
			maxCandidateCount:       10,
		},
	}
	obsConfig := agentMemoryObservabilityConfig{
		traceEnable:          true,
		traceSampleRatio:     0,
		traceMaxItems:        10,
		traceCapturePayload:  false,
		metricSampleDisabled: true,
	}
	candidates := []agentMemoryPipelineCandidate{
		{memoryType: "episodic", memoryID: 1, section: "facts", payload: "payload-a", lifecycleState: agentMemoryPipelineStateHot, estimatedTokens: 20, vectorScore: 0.9, recencyScore: 0.8, importanceScore: 0.7},
		{memoryType: "semantic", memoryID: 2, section: "facts", payload: "payload-b", lifecycleState: agentMemoryPipelineStateWarm, estimatedTokens: 20, vectorScore: 0.8, recencyScore: 0.7, importanceScore: 0.9},
	}

	result, err := observeAgentMemoryPipeline(candidates, pipelineConfig, obsConfig, sink, func() float64 { return 0 })
	require.NoError(t, err)
	require.Len(t, result.retrieved, 2)
	require.Len(t, result.assembly.selected, 2)
	require.Empty(t, sink.metrics)
	require.Empty(t, sink.traces)
}

func TestObserveAgentMemoryPipelineTraceVolumeStressCappedByMaxItems(t *testing.T) {
	sink := &agentMemoryObservationSinkMock{}
	const (
		candidateCount = 64
		traceMaxItems  = 7
	)
	pipelineConfig := agentMemoryPipelineConfig{
		includeArchived: false,
		retrieveLimitK:  candidateCount,
		retrieveWeights: agentMemoryRetrieveWeights{vectorWeight: 0.55, recencyWeight: 0.25, importanceWeight: 0.20},
		assemblyPolicy: agentMemoryContextAssemblyPolicy{
			totalBudgetTokens:       256,
			reservedOutputTokens:    0,
			budgetSafetyMarginRatio: 0,
			maxCandidateCount:       candidateCount,
		},
	}
	obsConfig := agentMemoryObservabilityConfig{
		traceEnable:          true,
		traceSampleRatio:     1,
		traceMaxItems:        traceMaxItems,
		traceCapturePayload:  false,
		metricSampleDisabled: false,
	}

	candidates := make([]agentMemoryPipelineCandidate, 0, candidateCount)
	for i := 0; i < candidateCount; i++ {
		candidates = append(candidates, agentMemoryPipelineCandidate{
			memoryType:      "episodic",
			memoryID:        uint64(i + 1),
			section:         "facts",
			payload:         "payload",
			lifecycleState:  agentMemoryPipelineStateHot,
			estimatedTokens: 1,
			vectorScore:     1,
			recencyScore:    1,
			importanceScore: 1,
		})
	}

	_, err := observeAgentMemoryPipeline(candidates, pipelineConfig, obsConfig, sink, func() float64 { return 0 })
	require.NoError(t, err)
	require.Len(t, sink.traces, 1)
	require.Equal(t, candidateCount, sink.traces[0].candidateCount)
	require.Len(t, sink.traces[0].items, traceMaxItems)
	for idx, item := range sink.traces[0].items {
		require.Equal(t, idx+1, item.rank)
	}
}

type agentMemoryObservationSinkMock struct {
	metrics []agentMemoryObservationMetrics
	traces  []agentMemoryTraceRecord
}

func (m *agentMemoryObservationSinkMock) recordMetrics(metrics agentMemoryObservationMetrics) {
	m.metrics = append(m.metrics, metrics)
}

func (m *agentMemoryObservationSinkMock) recordTrace(trace agentMemoryTraceRecord) {
	m.traces = append(m.traces, trace)
}
