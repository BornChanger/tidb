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
	"sort"
	"sync"
)

type agentMemoryAdapterOperation string

const (
	agentMemoryAdapterOpRetrieve agentMemoryAdapterOperation = "retrieve"
	agentMemoryAdapterOpUpsert   agentMemoryAdapterOperation = "upsert"
	agentMemoryAdapterOpAssemble agentMemoryAdapterOperation = "assemble"
	agentMemoryAdapterOpTrace    agentMemoryAdapterOperation = "trace"
)

type agentMemoryAdapterProfile struct {
	name           string
	version        int
	supportedOps   map[agentMemoryAdapterOperation]struct{}
	contractSource string
}

func newAgentMemoryAdapterProfile(name string, version int, ops []agentMemoryAdapterOperation) agentMemoryAdapterProfile {
	supportedOps := make(map[agentMemoryAdapterOperation]struct{}, len(ops))
	for _, op := range ops {
		supportedOps[op] = struct{}{}
	}
	return agentMemoryAdapterProfile{
		name:           name,
		version:        version,
		supportedOps:   supportedOps,
		contractSource: "sql",
	}
}

func negotiateAgentMemoryAdapterProfile(profiles []agentMemoryAdapterProfile, name string, requestedVersion int) (agentMemoryAdapterProfile, error) {
	matched := make([]agentMemoryAdapterProfile, 0, len(profiles))
	for _, profile := range profiles {
		if profile.name == name {
			matched = append(matched, profile)
		}
	}
	if len(matched) == 0 {
		return agentMemoryAdapterProfile{}, fmt.Errorf("adapter profile %s not found", name)
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].version < matched[j].version
	})

	chosen := matched[0]
	for _, profile := range matched {
		if profile.version <= requestedVersion && profile.version >= chosen.version {
			chosen = profile
		}
	}
	if chosen.version > requestedVersion {
		chosen = matched[0]
	}
	if requestedVersion >= matched[len(matched)-1].version {
		chosen = matched[len(matched)-1]
	}
	return chosen, nil
}

type agentMemoryAdapterConformanceReport struct {
	passed            bool
	checkedOperations []agentMemoryAdapterOperation
	endToEndPassed    bool
	checkedRequestIDs []string
}

type agentMemoryAdapterCompatibilityRow struct {
	profileName string
	version     int
	conformant  bool
	missingOps  []agentMemoryAdapterOperation
}

var agentMemoryAdapterRequiredOperations = []agentMemoryAdapterOperation{
	agentMemoryAdapterOpRetrieve,
	agentMemoryAdapterOpUpsert,
	agentMemoryAdapterOpAssemble,
	agentMemoryAdapterOpTrace,
}

func missingAgentMemoryAdapterOperations(profile agentMemoryAdapterProfile) []agentMemoryAdapterOperation {
	missing := make([]agentMemoryAdapterOperation, 0, len(agentMemoryAdapterRequiredOperations))
	for _, op := range agentMemoryAdapterRequiredOperations {
		if _, ok := profile.supportedOps[op]; !ok {
			missing = append(missing, op)
		}
	}
	return missing
}

func buildAgentMemoryAdapterCompatibilityMatrix(profiles []agentMemoryAdapterProfile) []agentMemoryAdapterCompatibilityRow {
	rows := make([]agentMemoryAdapterCompatibilityRow, 0, len(profiles))
	for _, profile := range profiles {
		missingOps := missingAgentMemoryAdapterOperations(profile)
		row := agentMemoryAdapterCompatibilityRow{
			profileName: profile.name,
			version:     profile.version,
			conformant:  len(missingOps) == 0,
		}
		if len(missingOps) > 0 {
			row.missingOps = make([]agentMemoryAdapterOperation, len(missingOps))
			copy(row.missingOps, missingOps)
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].profileName != rows[j].profileName {
			return rows[i].profileName < rows[j].profileName
		}
		return rows[i].version < rows[j].version
	})
	return rows
}

func runAgentMemoryAdapterConformance(profile agentMemoryAdapterProfile) (agentMemoryAdapterConformanceReport, error) {
	missingOps := missingAgentMemoryAdapterOperations(profile)
	if len(missingOps) > 0 {
		return agentMemoryAdapterConformanceReport{}, fmt.Errorf("missing required operation %s", missingOps[0])
	}
	checkedOps := make([]agentMemoryAdapterOperation, len(agentMemoryAdapterRequiredOperations))
	copy(checkedOps, agentMemoryAdapterRequiredOperations)
	return agentMemoryAdapterConformanceReport{
		passed:            true,
		checkedOperations: checkedOps,
	}, nil
}

func runAgentMemoryAdapterConformanceEndToEnd(profile agentMemoryAdapterProfile) (agentMemoryAdapterConformanceReport, error) {
	report, err := runAgentMemoryAdapterConformance(profile)
	if err != nil {
		return agentMemoryAdapterConformanceReport{}, err
	}
	type opStep struct {
		requestID string
		op        agentMemoryAdapterOperation
	}
	steps := []opStep{
		{requestID: "conformance-retrieve", op: agentMemoryAdapterOpRetrieve},
		{requestID: "conformance-upsert", op: agentMemoryAdapterOpUpsert},
		{requestID: "conformance-assemble", op: agentMemoryAdapterOpAssemble},
		{requestID: "conformance-trace", op: agentMemoryAdapterOpTrace},
	}
	runtime := newAgentMemoryAdapterRuntime(profile)
	checkedRequestIDs := make([]string, 0, len(steps))
	for _, step := range steps {
		resp, execErr := runtime.execute(agentMemoryAdapterRequest{requestID: step.requestID, operation: step.op})
		if execErr != nil {
			return agentMemoryAdapterConformanceReport{}, execErr
		}
		if resp.status != "ok" {
			return agentMemoryAdapterConformanceReport{}, fmt.Errorf("operation %s returned unexpected status %s", step.op, resp.status)
		}
		checkedRequestIDs = append(checkedRequestIDs, step.requestID)
	}
	retryResp, execErr := runtime.execute(agentMemoryAdapterRequest{requestID: "conformance-retrieve", operation: agentMemoryAdapterOpRetrieve})
	if execErr != nil {
		return agentMemoryAdapterConformanceReport{}, execErr
	}
	if retryResp.status != "ok" {
		return agentMemoryAdapterConformanceReport{}, fmt.Errorf("retry retrieve returned unexpected status %s", retryResp.status)
	}
	if runtime.executedCount("conformance-retrieve") != 1 {
		return agentMemoryAdapterConformanceReport{}, fmt.Errorf("retry retrieve should be idempotent")
	}
	report.endToEndPassed = true
	report.checkedRequestIDs = checkedRequestIDs
	return report, nil
}

type agentMemoryAdapterRequest struct {
	tenantID  string
	requestID string
	operation agentMemoryAdapterOperation
}

type agentMemoryAdapterRequestKey struct {
	tenantID  string
	requestID string
}

func makeAgentMemoryAdapterRequestKey(tenantID, requestID string) agentMemoryAdapterRequestKey {
	return agentMemoryAdapterRequestKey{tenantID: tenantID, requestID: requestID}
}

type agentMemoryAdapterResponse struct {
	status    string
	errorCode string
}

type agentMemoryAdapterRuntime struct {
	profile    agentMemoryAdapterProfile
	mu         sync.Mutex
	responses  map[agentMemoryAdapterRequestKey]agentMemoryAdapterResponse
	counters   map[agentMemoryAdapterRequestKey]int
	operations map[agentMemoryAdapterRequestKey]agentMemoryAdapterOperation
}

func newAgentMemoryAdapterRuntime(profile agentMemoryAdapterProfile) *agentMemoryAdapterRuntime {
	return &agentMemoryAdapterRuntime{
		profile:    profile,
		responses:  make(map[agentMemoryAdapterRequestKey]agentMemoryAdapterResponse),
		counters:   make(map[agentMemoryAdapterRequestKey]int),
		operations: make(map[agentMemoryAdapterRequestKey]agentMemoryAdapterOperation),
	}
}

func (r *agentMemoryAdapterRuntime) execute(req agentMemoryAdapterRequest) (agentMemoryAdapterResponse, error) {
	r.mu.Lock()
	key := makeAgentMemoryAdapterRequestKey(req.tenantID, req.requestID)
	if existingOp, ok := r.operations[key]; ok {
		if existingOp != req.operation {
			r.mu.Unlock()
			return agentMemoryAdapterResponse{}, fmt.Errorf("request %s operation %s conflicts with existing operation %s", req.requestID, req.operation, existingOp)
		}
	}
	if resp, ok := r.responses[key]; ok {
		r.mu.Unlock()
		return resp, nil
	}
	if _, ok := r.profile.supportedOps[req.operation]; !ok {
		r.mu.Unlock()
		return agentMemoryAdapterResponse{}, fmt.Errorf("operation %s is not supported by profile %s/v%d", req.operation, r.profile.name, r.profile.version)
	}
	resp := agentMemoryAdapterResponse{status: "ok"}
	r.responses[key] = resp
	r.counters[key] = 1
	r.operations[key] = req.operation
	r.mu.Unlock()
	return resp, nil
}

func (r *agentMemoryAdapterRuntime) executedCount(requestID string) int {
	r.mu.Lock()
	count := r.counters[makeAgentMemoryAdapterRequestKey("", requestID)]
	r.mu.Unlock()
	return count
}
