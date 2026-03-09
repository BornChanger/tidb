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

func TestBuildAgentMemoryExplainStages(t *testing.T) {
	trace := agentMemoryTraceRecord{
		traceID:        "trace-a",
		candidateCount: 10,
		selectedCount:  3,
		droppedCount:   7,
		errorCode:      "",
		warningFlags:   []string{"fallback_path"},
	}

	stages := buildAgentMemoryExplainStages(trace)
	require.Len(t, stages, 3)
	require.Equal(t, "candidate_generation", stages[0].stage)
	require.Equal(t, 10, stages[0].inputCount)
	require.Equal(t, 10, stages[0].outputCount)
	require.Equal(t, "ranking_filter", stages[1].stage)
	require.Equal(t, 10, stages[1].inputCount)
	require.Equal(t, 3, stages[1].outputCount)
	require.Equal(t, "context_assembly", stages[2].stage)
	require.Equal(t, 3, stages[2].inputCount)
	require.Equal(t, 3, stages[2].outputCount)
	require.Equal(t, "ok", stages[2].status)
}

func TestBuildAgentMemoryExplainStagesErrorPath(t *testing.T) {
	trace := agentMemoryTraceRecord{
		traceID:        "trace-b",
		candidateCount: 5,
		selectedCount:  0,
		droppedCount:   5,
		errorCode:      "invalid limit_k",
	}

	stages := buildAgentMemoryExplainStages(trace)
	require.Len(t, stages, 3)
	require.Equal(t, "error", stages[2].status)
	require.Contains(t, stages[2].detail, "invalid limit_k")
}

func TestRenderAgentMemoryExplainRowsStable(t *testing.T) {
	stages := []agentMemoryExplainStage{
		{stage: "candidate_generation", inputCount: 8, outputCount: 8, status: "ok", detail: ""},
		{stage: "ranking_filter", inputCount: 8, outputCount: 4, status: "ok", detail: ""},
		{stage: "context_assembly", inputCount: 4, outputCount: 3, status: "ok", detail: "dropped=1"},
	}
	rows := renderAgentMemoryExplainRows(stages)
	require.Equal(t, []string{
		"candidate_generation\t8\t8\tok\t",
		"ranking_filter\t8\t4\tok\t",
		"context_assembly\t4\t3\tok\tdropped=1",
	}, rows)
}

func TestQueryAgentMemoryTraceRowsStableOrder(t *testing.T) {
	records := []agentMemoryTraceRecord{
		{traceID: "trace-2", candidateCount: 5, selectedCount: 2, droppedCount: 3, createdAt: 100, warningFlags: []string{"w2", "w1"}},
		{traceID: "trace-1", candidateCount: 5, selectedCount: 1, droppedCount: 4, createdAt: 120, errorCode: "timeout"},
		{traceID: "trace-3", candidateCount: 6, selectedCount: 2, droppedCount: 4, createdAt: 100},
	}

	rows := queryAgentMemoryTraceRows(records, 2)
	require.Len(t, rows, 2)
	require.Equal(t, "trace-1", rows[0].traceID)
	require.Equal(t, "error", rows[0].status)
	require.Equal(t, 0.8, rows[0].dropRatio)
	require.Equal(t, "trace-2", rows[1].traceID)
	require.Equal(t, "ok", rows[1].status)
	require.Equal(t, "w1,w2", rows[1].warningFlags)
}

func TestBuildAgentMemoryTraceDiagnosticSurfaceStableFields(t *testing.T) {
	trace := agentMemoryTraceRecord{
		traceID:        "trace-9",
		candidateCount: 10,
		selectedCount:  3,
		droppedCount:   7,
		errorCode:      "timeout",
		warningFlags:   []string{"late_binding", "", "fallback_path"},
	}

	surface := buildAgentMemoryTraceDiagnosticSurface(trace)
	require.Equal(t, "error", surface.status)
	require.Equal(t, "timeout", surface.errorCode)
	require.Equal(t, "fallback_path,late_binding", surface.warningFlags)
	require.InDelta(t, 0.7, surface.dropRatio, 0.000001)
	require.Equal(t, 10, surface.candidateN)
	require.Equal(t, 3, surface.selectedK)
	require.Equal(t, []agentMemoryTraceDiagnosticField{
		{name: "status", value: "error"},
		{name: "error_code", value: "timeout"},
		{name: "warning_flags", value: "fallback_path,late_binding"},
		{name: "drop_ratio", value: "0.700000"},
		{name: "candidate_n", value: "10"},
		{name: "selected_k", value: "3"},
	}, surface.orderedFields())
}
