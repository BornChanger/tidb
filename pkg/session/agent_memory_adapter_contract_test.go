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

package session

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentMemoryAdapterVersionNegotiation(t *testing.T) {
	profiles := []agentMemoryAdapterProfile{
		newAgentMemoryAdapterProfile("mem0", 1, []agentMemoryAdapterOperation{agentMemoryAdapterOpRetrieve, agentMemoryAdapterOpUpsert, agentMemoryAdapterOpAssemble, agentMemoryAdapterOpTrace}),
		newAgentMemoryAdapterProfile("mem0", 2, []agentMemoryAdapterOperation{agentMemoryAdapterOpRetrieve, agentMemoryAdapterOpUpsert, agentMemoryAdapterOpAssemble, agentMemoryAdapterOpTrace}),
	}

	profile, err := negotiateAgentMemoryAdapterProfile(profiles, "mem0", 1)
	require.NoError(t, err)
	require.Equal(t, 1, profile.version)

	profile, err = negotiateAgentMemoryAdapterProfile(profiles, "mem0", 3)
	require.NoError(t, err)
	require.Equal(t, 2, profile.version)

	_, err = negotiateAgentMemoryAdapterProfile(profiles, "langgraph", 1)
	require.Error(t, err)
}

func TestAgentMemoryAdapterConformanceMem0Profile(t *testing.T) {
	profile := newAgentMemoryAdapterProfile("mem0", 1, []agentMemoryAdapterOperation{
		agentMemoryAdapterOpRetrieve,
		agentMemoryAdapterOpUpsert,
		agentMemoryAdapterOpAssemble,
		agentMemoryAdapterOpTrace,
	})
	report, err := runAgentMemoryAdapterConformance(profile)
	require.NoError(t, err)
	require.True(t, report.passed)
	require.Equal(t, "sql", profile.contractSource)
	require.Contains(t, report.checkedOperations, agentMemoryAdapterOpRetrieve)
	require.Contains(t, report.checkedOperations, agentMemoryAdapterOpTrace)
}

func TestAgentMemoryAdapterIdempotencyRetry(t *testing.T) {
	profile := newAgentMemoryAdapterProfile("mem0", 1, []agentMemoryAdapterOperation{
		agentMemoryAdapterOpRetrieve,
		agentMemoryAdapterOpUpsert,
		agentMemoryAdapterOpAssemble,
		agentMemoryAdapterOpTrace,
	})
	runtime := newAgentMemoryAdapterRuntime(profile)

	resp1, err := runtime.execute(agentMemoryAdapterRequest{requestID: "req-1", operation: agentMemoryAdapterOpRetrieve})
	require.NoError(t, err)
	require.Equal(t, "ok", resp1.status)

	resp2, err := runtime.execute(agentMemoryAdapterRequest{requestID: "req-1", operation: agentMemoryAdapterOpRetrieve})
	require.NoError(t, err)
	require.Equal(t, resp1, resp2)
	require.Equal(t, 1, runtime.executedCount("req-1"))
}

func TestAgentMemoryAdapterIdempotencyRequestIDIsolationAcrossTenants(t *testing.T) {
	profile := newAgentMemoryAdapterProfile("mem0", 1, []agentMemoryAdapterOperation{
		agentMemoryAdapterOpRetrieve,
		agentMemoryAdapterOpUpsert,
		agentMemoryAdapterOpAssemble,
		agentMemoryAdapterOpTrace,
	})
	runtime := newAgentMemoryAdapterRuntime(profile)

	_, err := runtime.execute(agentMemoryAdapterRequest{tenantID: "tenant-a", requestID: "req-1", operation: agentMemoryAdapterOpRetrieve})
	require.NoError(t, err)

	_, err = runtime.execute(agentMemoryAdapterRequest{tenantID: "tenant-b", requestID: "req-1", operation: agentMemoryAdapterOpUpsert})
	require.NoError(t, err)

	_, err = runtime.execute(agentMemoryAdapterRequest{tenantID: "tenant-a", requestID: "req-1", operation: agentMemoryAdapterOpRetrieve})
	require.NoError(t, err)

	_, err = runtime.execute(agentMemoryAdapterRequest{tenantID: "tenant-b", requestID: "req-1", operation: agentMemoryAdapterOpUpsert})
	require.NoError(t, err)

	require.Len(t, runtime.responses, 2)
	require.Len(t, runtime.counters, 2)
	require.Len(t, runtime.operations, 2)
}

func TestAgentMemoryAdapterIdempotencyRequestIDConflictAcrossOperations(t *testing.T) {
	profile := newAgentMemoryAdapterProfile("mem0", 1, []agentMemoryAdapterOperation{
		agentMemoryAdapterOpRetrieve,
		agentMemoryAdapterOpUpsert,
		agentMemoryAdapterOpAssemble,
		agentMemoryAdapterOpTrace,
	})
	runtime := newAgentMemoryAdapterRuntime(profile)

	_, err := runtime.execute(agentMemoryAdapterRequest{requestID: "req-1", operation: agentMemoryAdapterOpRetrieve})
	require.NoError(t, err)

	_, err = runtime.execute(agentMemoryAdapterRequest{requestID: "req-1", operation: agentMemoryAdapterOpUpsert})
	require.Error(t, err)
	require.Contains(t, err.Error(), "conflicts with existing operation")
	require.Equal(t, 1, runtime.executedCount("req-1"))
}

func TestAgentMemoryAdapterCompatibilityMatrix(t *testing.T) {
	profiles := []agentMemoryAdapterProfile{
		newAgentMemoryAdapterProfile("mem0", 2, []agentMemoryAdapterOperation{agentMemoryAdapterOpRetrieve, agentMemoryAdapterOpUpsert, agentMemoryAdapterOpAssemble, agentMemoryAdapterOpTrace}),
		newAgentMemoryAdapterProfile("mem0", 1, []agentMemoryAdapterOperation{agentMemoryAdapterOpRetrieve, agentMemoryAdapterOpUpsert, agentMemoryAdapterOpAssemble}),
		newAgentMemoryAdapterProfile("mcp", 1, []agentMemoryAdapterOperation{agentMemoryAdapterOpRetrieve, agentMemoryAdapterOpTrace}),
	}
	matrix := buildAgentMemoryAdapterCompatibilityMatrix(profiles)
	require.Len(t, matrix, 3)

	byVersion := make(map[string]agentMemoryAdapterCompatibilityRow, len(matrix))
	for _, row := range matrix {
		byVersion[fmt.Sprintf("%s/v%d", row.profileName, row.version)] = row
	}

	rowMem01 := byVersion["mem0/v1"]
	require.False(t, rowMem01.conformant)
	require.ElementsMatch(t, []agentMemoryAdapterOperation{agentMemoryAdapterOpTrace}, rowMem01.missingOps)

	rowMem02 := byVersion["mem0/v2"]
	require.True(t, rowMem02.conformant)
	require.Empty(t, rowMem02.missingOps)

	rowMCP := byVersion["mcp/v1"]
	require.False(t, rowMCP.conformant)
	require.ElementsMatch(t, []agentMemoryAdapterOperation{agentMemoryAdapterOpUpsert, agentMemoryAdapterOpAssemble}, rowMCP.missingOps)
}

func TestAgentMemoryAdapterConformanceCheckedOperationsImmutable(t *testing.T) {
	profile := newAgentMemoryAdapterProfile("mem0", 1, []agentMemoryAdapterOperation{
		agentMemoryAdapterOpRetrieve,
		agentMemoryAdapterOpUpsert,
		agentMemoryAdapterOpAssemble,
		agentMemoryAdapterOpTrace,
	})
	report1, err := runAgentMemoryAdapterConformance(profile)
	require.NoError(t, err)
	require.Len(t, report1.checkedOperations, 4)

	report1.checkedOperations[0] = agentMemoryAdapterOpTrace

	report2, err := runAgentMemoryAdapterConformance(profile)
	require.NoError(t, err)
	require.Equal(t, agentMemoryAdapterOpRetrieve, report2.checkedOperations[0])
}

func TestAgentMemoryAdapterConformanceEndToEnd(t *testing.T) {
	profile := newAgentMemoryAdapterProfile("mem0", 1, []agentMemoryAdapterOperation{
		agentMemoryAdapterOpRetrieve,
		agentMemoryAdapterOpUpsert,
		agentMemoryAdapterOpAssemble,
		agentMemoryAdapterOpTrace,
	})
	report, err := runAgentMemoryAdapterConformanceEndToEnd(profile)
	require.NoError(t, err)
	require.True(t, report.passed)
	require.True(t, report.endToEndPassed)
	require.Len(t, report.checkedRequestIDs, 4)
	require.Contains(t, report.checkedRequestIDs, "conformance-retrieve")
	require.Contains(t, report.checkedRequestIDs, "conformance-upsert")
	require.Contains(t, report.checkedRequestIDs, "conformance-assemble")
	require.Contains(t, report.checkedRequestIDs, "conformance-trace")
}

func TestAgentMemoryAdapterConformanceEndToEndRejectsMissingOps(t *testing.T) {
	profile := newAgentMemoryAdapterProfile("mem0", 1, []agentMemoryAdapterOperation{
		agentMemoryAdapterOpRetrieve,
		agentMemoryAdapterOpUpsert,
		agentMemoryAdapterOpAssemble,
	})
	_, err := runAgentMemoryAdapterConformanceEndToEnd(profile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required operation")
}
