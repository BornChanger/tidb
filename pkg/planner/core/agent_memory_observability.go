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
	"time"
)

const agentMemoryTraceDefaultMaxItems = 100

type agentMemoryObservabilityConfig struct {
	traceEnable          bool
	traceSampleRatio     float64
	traceMaxItems        int
	traceCapturePayload  bool
	metricSampleDisabled bool
}

type agentMemoryObservationMetrics struct {
	operation      string
	status         string
	candidateCount int
	selectedCount  int
	droppedCount   int
	dropRatio      float64
	errorCode      string
}

type agentMemoryTraceItem struct {
	memoryType      string
	memoryID        uint64
	rank            int
	selected        bool
	droppedReason   string
	vectorScore     float64
	recencyScore    float64
	importanceScore float64
	finalScore      float64
	payload         string
}

type agentMemoryTraceRecord struct {
	traceID        string
	createdAt      int64
	candidateCount int
	selectedCount  int
	droppedCount   int
	errorCode      string
	warningFlags   []string
	items          []agentMemoryTraceItem
}

type agentMemoryObservationSink interface {
	recordMetrics(metrics agentMemoryObservationMetrics)
	recordTrace(trace agentMemoryTraceRecord)
}

func observeAgentMemoryPipeline(
	candidates []agentMemoryPipelineCandidate,
	pipelineConfig agentMemoryPipelineConfig,
	obsConfig agentMemoryObservabilityConfig,
	sink agentMemoryObservationSink,
	sampleRand func() float64,
) (agentMemoryPipelineResult, error) {
	result, err := runAgentMemoryPipeline(candidates, pipelineConfig)
	candidateCount := len(candidates)
	selectedCount := 0
	if err == nil {
		selectedCount = len(result.assembly.selected)
	}
	droppedCount := candidateCount - selectedCount
	if droppedCount < 0 {
		droppedCount = 0
	}
	dropRatio := 0.0
	if candidateCount > 0 {
		dropRatio = float64(droppedCount) / float64(candidateCount)
	}
	status := "ok"
	errorCode := ""
	if err != nil {
		status = "error"
		errorCode = err.Error()
	}

	if sink != nil && !obsConfig.metricSampleDisabled {
		sink.recordMetrics(agentMemoryObservationMetrics{
			operation:      "retrieve_assemble",
			status:         status,
			candidateCount: candidateCount,
			selectedCount:  selectedCount,
			droppedCount:   droppedCount,
			dropRatio:      dropRatio,
			errorCode:      errorCode,
		})
	}

	if sink == nil || !obsConfig.traceEnable || !shouldSampleAgentMemoryTrace(err, obsConfig.traceSampleRatio, sampleRand) {
		return result, err
	}

	maxTraceItems := obsConfig.traceMaxItems
	if maxTraceItems <= 0 {
		maxTraceItems = agentMemoryTraceDefaultMaxItems
	}

	payloadByID := make(map[agentMemoryPipelineIdentity]string, len(candidates))
	for _, candidate := range candidates {
		payloadByID[agentMemoryPipelineIdentity{memoryType: candidate.memoryType, memoryID: candidate.memoryID}] = candidate.payload
	}
	retrievedByID := make(map[agentMemoryPipelineIdentity]agentMemoryPipelineRetrieved, len(result.retrieved))
	for _, item := range result.retrieved {
		retrievedByID[agentMemoryPipelineIdentity{memoryType: item.memoryType, memoryID: item.memoryID}] = item
	}
	items := make([]agentMemoryTraceItem, 0, maxTraceItems)
	for _, manifestItem := range result.assembly.manifest {
		if len(items) >= maxTraceItems {
			break
		}
		identity := agentMemoryPipelineIdentity{memoryType: manifestItem.memoryType, memoryID: manifestItem.memoryID}
		retrievedItem := retrievedByID[identity]
		payload := ""
		if obsConfig.traceCapturePayload {
			payload = payloadByID[identity]
		}
		items = append(items, agentMemoryTraceItem{
			memoryType:      manifestItem.memoryType,
			memoryID:        manifestItem.memoryID,
			rank:            manifestItem.rank,
			selected:        manifestItem.decision == "included",
			droppedReason:   manifestItem.reason,
			vectorScore:     retrievedItem.vectorScore,
			recencyScore:    retrievedItem.recencyScore,
			importanceScore: retrievedItem.importanceScore,
			finalScore:      retrievedItem.finalScore,
			payload:         payload,
		})
	}

	trace := agentMemoryTraceRecord{
		traceID:        fmt.Sprintf("am-trace-%d", time.Now().UnixNano()),
		createdAt:      time.Now().UnixNano(),
		candidateCount: candidateCount,
		selectedCount:  selectedCount,
		droppedCount:   droppedCount,
		errorCode:      errorCode,
		items:          items,
	}
	if err != nil {
		trace.warningFlags = []string{"error_path"}
	}
	sink.recordTrace(trace)
	return result, err
}

func shouldSampleAgentMemoryTrace(err error, ratio float64, sampleRand func() float64) bool {
	if err != nil {
		return true
	}
	if ratio <= 0 {
		return false
	}
	if ratio >= 1 {
		return true
	}
	if sampleRand == nil {
		return false
	}
	return sampleRand() < ratio
}
