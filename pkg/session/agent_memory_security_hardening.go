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
	"strings"
	"sync"
)

type agentMemoryRole string

const (
	agentMemoryRoleDefault    agentMemoryRole = "default"
	agentMemoryRolePrivileged agentMemoryRole = "privileged"
)

type agentMemoryClassification string

const (
	agentMemoryClassificationPublic       agentMemoryClassification = "public"
	agentMemoryClassificationConfidential agentMemoryClassification = "confidential"
)

type agentMemorySecurityItem struct {
	tenantID       string
	memoryID       uint64
	classification agentMemoryClassification
	payload        string
}

func applyAgentMemoryRedaction(items []agentMemorySecurityItem, role agentMemoryRole, allowSensitive bool) []agentMemorySecurityItem {
	redacted := make([]agentMemorySecurityItem, 0, len(items))
	for _, item := range items {
		payload := item.payload
		if item.classification == agentMemoryClassificationConfidential && !(role == agentMemoryRolePrivileged && allowSensitive) {
			payload = "[REDACTED]"
		}
		redacted = append(redacted, agentMemorySecurityItem{
			tenantID:       item.tenantID,
			memoryID:       item.memoryID,
			classification: item.classification,
			payload:        payload,
		})
	}
	return redacted
}

func filterAgentMemoryByTenant(items []agentMemorySecurityItem, tenantID string) []agentMemorySecurityItem {
	if strings.TrimSpace(tenantID) == "" {
		return []agentMemorySecurityItem{}
	}
	filtered := make([]agentMemorySecurityItem, 0, len(items))
	for _, item := range items {
		if item.tenantID != tenantID {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func applyAgentMemorySecurityPolicy(items []agentMemorySecurityItem, tenantID string, role agentMemoryRole, allowSensitive bool) []agentMemorySecurityItem {
	tenantScoped := filterAgentMemoryByTenant(items, tenantID)
	return applyAgentMemoryRedaction(tenantScoped, role, allowSensitive)
}

func validateAgentMemoryPrivilegedAccessReason(role agentMemoryRole, reason string) error {
	if role != agentMemoryRolePrivileged {
		return nil
	}
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("privileged access requires explicit audit reason")
	}
	return nil
}

type agentMemoryPurgeCheckpoint struct {
	steps         []string
	completed     map[string]struct{}
	nextStepIndex int
	finalized     bool
}

type agentMemoryPurgeWorkflow struct {
	mu          sync.Mutex
	checkpoints map[string]agentMemoryPurgeCheckpoint
}

func newAgentMemoryPurgeWorkflow() *agentMemoryPurgeWorkflow {
	return &agentMemoryPurgeWorkflow{checkpoints: make(map[string]agentMemoryPurgeCheckpoint)}
}

func (w *agentMemoryPurgeWorkflow) start(jobID string, steps []string) error {
	if len(steps) == 0 {
		return fmt.Errorf("purge steps should not be empty")
	}
	w.mu.Lock()
	if _, ok := w.checkpoints[jobID]; ok {
		w.mu.Unlock()
		return fmt.Errorf("job %s already exists", jobID)
	}
	stepsCopy := make([]string, len(steps))
	copy(stepsCopy, steps)
	w.checkpoints[jobID] = agentMemoryPurgeCheckpoint{
		steps:         stepsCopy,
		completed:     make(map[string]struct{}),
		nextStepIndex: 0,
	}
	w.mu.Unlock()
	return nil
}

func (w *agentMemoryPurgeWorkflow) nextPendingStep(jobID string) string {
	w.mu.Lock()
	checkpoint, ok := w.checkpoints[jobID]
	if !ok || checkpoint.nextStepIndex >= len(checkpoint.steps) {
		w.mu.Unlock()
		return ""
	}
	step := checkpoint.steps[checkpoint.nextStepIndex]
	w.mu.Unlock()
	return step
}

func (w *agentMemoryPurgeWorkflow) completeStep(jobID string, step string) error {
	w.mu.Lock()
	checkpoint, ok := w.checkpoints[jobID]
	if !ok {
		w.mu.Unlock()
		return fmt.Errorf("job %s not found", jobID)
	}
	if checkpoint.finalized {
		w.mu.Unlock()
		return fmt.Errorf("job %s already finalized", jobID)
	}
	if checkpoint.nextStepIndex >= len(checkpoint.steps) {
		checkpoint.finalized = true
		w.checkpoints[jobID] = checkpoint
		w.mu.Unlock()
		return nil
	}
	expectedStep := checkpoint.steps[checkpoint.nextStepIndex]
	if expectedStep != step {
		w.mu.Unlock()
		return fmt.Errorf("unexpected step %s, expected %s", step, expectedStep)
	}
	checkpoint.completed[step] = struct{}{}
	checkpoint.nextStepIndex++
	if checkpoint.nextStepIndex >= len(checkpoint.steps) {
		checkpoint.finalized = true
	}
	w.checkpoints[jobID] = checkpoint
	w.mu.Unlock()
	return nil
}

func (w *agentMemoryPurgeWorkflow) getCheckpoint(jobID string) (agentMemoryPurgeCheckpoint, bool) {
	w.mu.Lock()
	checkpoint, ok := w.checkpoints[jobID]
	if !ok {
		w.mu.Unlock()
		return agentMemoryPurgeCheckpoint{}, false
	}
	cloned := cloneAgentMemoryPurgeCheckpoint(checkpoint)
	w.mu.Unlock()
	return cloned, true
}

func (w *agentMemoryPurgeWorkflow) restoreCheckpoint(jobID string, checkpoint agentMemoryPurgeCheckpoint) {
	w.mu.Lock()
	w.checkpoints[jobID] = cloneAgentMemoryPurgeCheckpoint(checkpoint)
	w.mu.Unlock()
}

func cloneAgentMemoryPurgeCheckpoint(checkpoint agentMemoryPurgeCheckpoint) agentMemoryPurgeCheckpoint {
	steps := make([]string, len(checkpoint.steps))
	copy(steps, checkpoint.steps)
	completed := make(map[string]struct{}, len(checkpoint.completed))
	for step := range checkpoint.completed {
		completed[step] = struct{}{}
	}
	return agentMemoryPurgeCheckpoint{
		steps:         steps,
		completed:     completed,
		nextStepIndex: checkpoint.nextStepIndex,
		finalized:     checkpoint.finalized,
	}
}
