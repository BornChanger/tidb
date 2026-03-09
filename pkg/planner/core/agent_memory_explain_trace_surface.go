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
	"sort"
	"strings"
)

type agentMemoryExplainStage struct {
	stage       string
	inputCount  int
	outputCount int
	status      string
	detail      string
}

func buildAgentMemoryExplainStages(trace agentMemoryTraceRecord) []agentMemoryExplainStage {
	stages := []agentMemoryExplainStage{
		{
			stage:       "candidate_generation",
			inputCount:  trace.candidateCount,
			outputCount: trace.candidateCount,
			status:      "ok",
			detail:      strings.Join(normalizeWarningFlags(trace.warningFlags), ","),
		},
		{
			stage:       "ranking_filter",
			inputCount:  trace.candidateCount,
			outputCount: trace.selectedCount,
			status:      "ok",
			detail:      fmt.Sprintf("dropped=%d", trace.droppedCount),
		},
		{
			stage:       "context_assembly",
			inputCount:  trace.selectedCount,
			outputCount: trace.selectedCount,
			status:      "ok",
			detail:      "",
		},
	}
	if trace.errorCode != "" {
		stages[2].status = "error"
		stages[2].detail = trace.errorCode
	}
	return stages
}

func renderAgentMemoryExplainRows(stages []agentMemoryExplainStage) []string {
	rows := make([]string, 0, len(stages))
	for _, stage := range stages {
		rows = append(rows, fmt.Sprintf("%s\t%d\t%d\t%s\t%s", stage.stage, stage.inputCount, stage.outputCount, stage.status, stage.detail))
	}
	return rows
}

type agentMemoryTraceQueryRow struct {
	traceID      string
	candidateN   int
	selectedK    int
	dropRatio    float64
	status       string
	errorCode    string
	warningFlags string
}

type agentMemoryTraceDiagnosticField struct {
	name  string
	value string
}

type agentMemoryTraceDiagnosticSurface struct {
	status       string
	errorCode    string
	warningFlags string
	dropRatio    float64
	candidateN   int
	selectedK    int
}

func buildAgentMemoryTraceDiagnosticSurface(record agentMemoryTraceRecord) agentMemoryTraceDiagnosticSurface {
	surface := agentMemoryTraceDiagnosticSurface{
		status:       "ok",
		errorCode:    record.errorCode,
		warningFlags: strings.Join(normalizeWarningFlags(record.warningFlags), ","),
		dropRatio:    0,
		candidateN:   record.candidateCount,
		selectedK:    record.selectedCount,
	}
	if record.errorCode != "" {
		surface.status = "error"
	}
	if record.candidateCount > 0 {
		surface.dropRatio = float64(record.droppedCount) / float64(record.candidateCount)
	}
	return surface
}

func (surface agentMemoryTraceDiagnosticSurface) orderedFields() []agentMemoryTraceDiagnosticField {
	return []agentMemoryTraceDiagnosticField{
		{name: "status", value: surface.status},
		{name: "error_code", value: surface.errorCode},
		{name: "warning_flags", value: surface.warningFlags},
		{name: "drop_ratio", value: fmt.Sprintf("%.6f", surface.dropRatio)},
		{name: "candidate_n", value: fmt.Sprintf("%d", surface.candidateN)},
		{name: "selected_k", value: fmt.Sprintf("%d", surface.selectedK)},
	}
}

func queryAgentMemoryTraceRows(records []agentMemoryTraceRecord, limit int) []agentMemoryTraceQueryRow {
	if limit <= 0 || len(records) == 0 {
		return nil
	}
	recordsCopy := make([]agentMemoryTraceRecord, len(records))
	copy(recordsCopy, records)
	sort.SliceStable(recordsCopy, func(i, j int) bool {
		if recordsCopy[i].createdAt != recordsCopy[j].createdAt {
			return recordsCopy[i].createdAt > recordsCopy[j].createdAt
		}
		return recordsCopy[i].traceID < recordsCopy[j].traceID
	})
	if len(recordsCopy) > limit {
		recordsCopy = recordsCopy[:limit]
	}
	rows := make([]agentMemoryTraceQueryRow, 0, len(recordsCopy))
	for _, record := range recordsCopy {
		surface := buildAgentMemoryTraceDiagnosticSurface(record)
		rows = append(rows, agentMemoryTraceQueryRow{
			traceID:      record.traceID,
			candidateN:   surface.candidateN,
			selectedK:    surface.selectedK,
			dropRatio:    surface.dropRatio,
			status:       surface.status,
			errorCode:    surface.errorCode,
			warningFlags: surface.warningFlags,
		})
	}
	return rows
}

func normalizeWarningFlags(warnings []string) []string {
	if len(warnings) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		if warning == "" {
			continue
		}
		normalized = append(normalized, warning)
	}
	sort.Strings(normalized)
	return normalized
}
