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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyAgentMemoryRedactionByRole(t *testing.T) {
	items := []agentMemorySecurityItem{
		{memoryID: 1, classification: agentMemoryClassificationPublic, payload: "public-text"},
		{memoryID: 2, classification: agentMemoryClassificationConfidential, payload: "secret-text"},
	}

	redacted := applyAgentMemoryRedaction(items, agentMemoryRoleDefault, false)
	require.Equal(t, "public-text", redacted[0].payload)
	require.Equal(t, "[REDACTED]", redacted[1].payload)

	unredacted := applyAgentMemoryRedaction(items, agentMemoryRolePrivileged, true)
	require.Equal(t, "secret-text", unredacted[1].payload)
}

func TestValidateAgentMemoryPrivilegedAccessReason(t *testing.T) {
	err := validateAgentMemoryPrivilegedAccessReason(agentMemoryRolePrivileged, "")
	require.Error(t, err)
	err = validateAgentMemoryPrivilegedAccessReason(agentMemoryRolePrivileged, "incident-debug")
	require.NoError(t, err)
	err = validateAgentMemoryPrivilegedAccessReason(agentMemoryRoleDefault, "")
	require.NoError(t, err)
}

func TestAgentMemoryPurgeWorkflowCheckpointRecovery(t *testing.T) {
	workflow := newAgentMemoryPurgeWorkflow()
	steps := []string{"erase_payload", "cleanup_artifacts", "final_audit"}
	require.NoError(t, workflow.start("job-1", steps))
	require.Equal(t, "erase_payload", workflow.nextPendingStep("job-1"))
	require.NoError(t, workflow.completeStep("job-1", "erase_payload"))

	checkpoint, ok := workflow.getCheckpoint("job-1")
	require.True(t, ok)
	require.Equal(t, 1, checkpoint.nextStepIndex)

	restored := newAgentMemoryPurgeWorkflow()
	restored.restoreCheckpoint("job-1", checkpoint)
	require.Equal(t, "cleanup_artifacts", restored.nextPendingStep("job-1"))
	require.NoError(t, restored.completeStep("job-1", "cleanup_artifacts"))
	require.NoError(t, restored.completeStep("job-1", "final_audit"))
	checkpoint, ok = restored.getCheckpoint("job-1")
	require.True(t, ok)
	require.True(t, checkpoint.finalized)
}

func TestApplyAgentMemorySecurityPolicyTenantIsolation(t *testing.T) {
	items := []agentMemorySecurityItem{
		{tenantID: "tenant-a", memoryID: 1, classification: agentMemoryClassificationPublic, payload: "public-a"},
		{tenantID: "tenant-a", memoryID: 2, classification: agentMemoryClassificationConfidential, payload: "secret-a"},
		{tenantID: "tenant-b", memoryID: 3, classification: agentMemoryClassificationConfidential, payload: "secret-b"},
	}

	secured := applyAgentMemorySecurityPolicy(items, "tenant-a", agentMemoryRoleDefault, false)
	require.Len(t, secured, 2)
	require.Equal(t, "tenant-a", secured[0].tenantID)
	require.Equal(t, "tenant-a", secured[1].tenantID)
	require.Equal(t, "public-a", secured[0].payload)
	require.Equal(t, "[REDACTED]", secured[1].payload)
}

func TestApplyAgentMemorySecurityPolicyPrivilegedTenantScoped(t *testing.T) {
	items := []agentMemorySecurityItem{
		{tenantID: "tenant-a", memoryID: 1, classification: agentMemoryClassificationConfidential, payload: "secret-a"},
		{tenantID: "tenant-b", memoryID: 2, classification: agentMemoryClassificationConfidential, payload: "secret-b"},
	}

	secured := applyAgentMemorySecurityPolicy(items, "tenant-a", agentMemoryRolePrivileged, true)
	require.Len(t, secured, 1)
	require.Equal(t, "tenant-a", secured[0].tenantID)
	require.Equal(t, "secret-a", secured[0].payload)
}

func TestAgentMemoryPurgeWorkflowConcurrentAccessFinalization(t *testing.T) {
	workflow := newAgentMemoryPurgeWorkflow()
	jobID := "job-concurrent"
	steps := []string{"erase_payload", "cleanup_artifacts", "final_audit"}
	require.NoError(t, workflow.start(jobID, steps))

	require.NoError(t, runConcurrentPurgeWorkers(workflow, jobID, steps, 8))

	checkpoint, ok := workflow.getCheckpoint(jobID)
	require.True(t, ok)
	require.True(t, checkpoint.finalized)
	require.Equal(t, len(steps), checkpoint.nextStepIndex)
	require.Len(t, checkpoint.completed, len(steps))
	for _, step := range steps {
		_, completed := checkpoint.completed[step]
		require.True(t, completed)
	}
	require.Equal(t, "", workflow.nextPendingStep(jobID))
}

func runConcurrentPurgeWorkers(workflow *agentMemoryPurgeWorkflow, jobID string, steps []string, workerCount int) error {
	if workerCount <= 0 {
		return fmt.Errorf("worker count must be positive")
	}
	allowedSteps := make(map[string]struct{}, len(steps))
	for _, step := range steps {
		allowedSteps[step] = struct{}{}
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	errCh := make(chan error, workerCount)

	for workerID := 0; workerID < workerCount; workerID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-start
			for {
				step := workflow.nextPendingStep(jobID)
				if step == "" {
					return
				}
				if _, ok := allowedSteps[step]; !ok {
					errCh <- fmt.Errorf("worker %d observed unknown step %q", id, step)
					return
				}
				err := workflow.completeStep(jobID, step)
				if err == nil {
					continue
				}
				if strings.Contains(err.Error(), "unexpected step") || strings.Contains(err.Error(), "already finalized") {
					continue
				}
				errCh <- fmt.Errorf("worker %d completeStep(%s): %w", id, step, err)
				return
			}
		}(workerID)
	}

	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}
