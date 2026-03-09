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
	"errors"
	"sync"
)

var (
	errAgentMemoryLifecycleJobPaused   = errors.New("agent-memory lifecycle job is paused")
	errAgentMemoryLifecycleJobCanceled = errors.New("agent-memory lifecycle job is canceled")
)

type agentMemoryLifecycleState string

const (
	agentMemoryLifecycleStateHot     agentMemoryLifecycleState = "hot"
	agentMemoryLifecycleStateWarm    agentMemoryLifecycleState = "warm"
	agentMemoryLifecycleStateCold    agentMemoryLifecycleState = "cold"
	agentMemoryLifecycleStateArchive agentMemoryLifecycleState = "archived"
)

type agentMemoryLifecycleTask struct {
	taskID      string
	memoryID    uint64
	targetState agentMemoryLifecycleState
}

type agentMemoryLifecycleMemoryItem struct {
	memoryID       uint64
	lifecycleState agentMemoryLifecycleState
}

type agentMemoryLifecycleCheckpoint struct {
	nextIndex        int
	completedTaskIDs map[string]struct{}
	paused           bool
	canceled         bool
}

type agentMemoryLifecycleScheduler struct {
	mu          sync.Mutex
	checkpoints map[string]agentMemoryLifecycleCheckpoint
}

func newAgentMemoryLifecycleScheduler() *agentMemoryLifecycleScheduler {
	return &agentMemoryLifecycleScheduler{
		checkpoints: make(map[string]agentMemoryLifecycleCheckpoint),
	}
}

func (s *agentMemoryLifecycleScheduler) runJob(jobID string, tasks []agentMemoryLifecycleTask, apply func(task agentMemoryLifecycleTask) error) (agentMemoryLifecycleCheckpoint, error) {
	s.mu.Lock()
	checkpoint := s.ensureCheckpointLocked(jobID)
	if checkpoint.canceled {
		result := cloneAgentMemoryLifecycleCheckpoint(checkpoint)
		s.mu.Unlock()
		return result, errAgentMemoryLifecycleJobCanceled
	}
	if checkpoint.paused {
		result := cloneAgentMemoryLifecycleCheckpoint(checkpoint)
		s.mu.Unlock()
		return result, errAgentMemoryLifecycleJobPaused
	}
	startIndex := checkpoint.nextIndex
	s.mu.Unlock()

	if startIndex < 0 {
		startIndex = 0
	}
	for idx := startIndex; idx < len(tasks); idx++ {
		task := tasks[idx]

		s.mu.Lock()
		checkpoint = s.ensureCheckpointLocked(jobID)
		if checkpoint.canceled {
			result := cloneAgentMemoryLifecycleCheckpoint(checkpoint)
			s.mu.Unlock()
			return result, errAgentMemoryLifecycleJobCanceled
		}
		if checkpoint.paused {
			result := cloneAgentMemoryLifecycleCheckpoint(checkpoint)
			s.mu.Unlock()
			return result, errAgentMemoryLifecycleJobPaused
		}
		if _, done := checkpoint.completedTaskIDs[task.taskID]; done {
			checkpoint.nextIndex = idx + 1
			s.checkpoints[jobID] = checkpoint
			s.mu.Unlock()
			continue
		}
		s.mu.Unlock()

		if err := apply(task); err != nil {
			s.mu.Lock()
			checkpoint = s.ensureCheckpointLocked(jobID)
			checkpoint.nextIndex = idx
			s.checkpoints[jobID] = checkpoint
			result := cloneAgentMemoryLifecycleCheckpoint(checkpoint)
			s.mu.Unlock()
			return result, err
		}

		s.mu.Lock()
		checkpoint = s.ensureCheckpointLocked(jobID)
		checkpoint.completedTaskIDs[task.taskID] = struct{}{}
		checkpoint.nextIndex = idx + 1
		s.checkpoints[jobID] = checkpoint
		s.mu.Unlock()
	}

	s.mu.Lock()
	checkpoint = s.ensureCheckpointLocked(jobID)
	result := cloneAgentMemoryLifecycleCheckpoint(checkpoint)
	s.mu.Unlock()
	return result, nil
}

func (s *agentMemoryLifecycleScheduler) pauseJob(jobID string) error {
	s.mu.Lock()
	checkpoint := s.ensureCheckpointLocked(jobID)
	if checkpoint.canceled {
		s.mu.Unlock()
		return errAgentMemoryLifecycleJobCanceled
	}
	checkpoint.paused = true
	s.checkpoints[jobID] = checkpoint
	s.mu.Unlock()
	return nil
}

func (s *agentMemoryLifecycleScheduler) resumeJob(jobID string) error {
	s.mu.Lock()
	checkpoint := s.ensureCheckpointLocked(jobID)
	if checkpoint.canceled {
		s.mu.Unlock()
		return errAgentMemoryLifecycleJobCanceled
	}
	checkpoint.paused = false
	s.checkpoints[jobID] = checkpoint
	s.mu.Unlock()
	return nil
}

func (s *agentMemoryLifecycleScheduler) cancelJob(jobID string) error {
	s.mu.Lock()
	checkpoint := s.ensureCheckpointLocked(jobID)
	checkpoint.canceled = true
	checkpoint.paused = false
	s.checkpoints[jobID] = checkpoint
	s.mu.Unlock()
	return nil
}

func (s *agentMemoryLifecycleScheduler) getCheckpoint(jobID string) (agentMemoryLifecycleCheckpoint, bool) {
	s.mu.Lock()
	checkpoint, ok := s.checkpoints[jobID]
	if !ok {
		s.mu.Unlock()
		return agentMemoryLifecycleCheckpoint{}, false
	}
	result := cloneAgentMemoryLifecycleCheckpoint(checkpoint)
	s.mu.Unlock()
	return result, true
}

func (s *agentMemoryLifecycleScheduler) restoreCheckpoint(jobID string, checkpoint agentMemoryLifecycleCheckpoint) {
	s.mu.Lock()
	s.checkpoints[jobID] = cloneAgentMemoryLifecycleCheckpoint(checkpoint)
	s.mu.Unlock()
}

func (s *agentMemoryLifecycleScheduler) ensureCheckpointLocked(jobID string) agentMemoryLifecycleCheckpoint {
	checkpoint, ok := s.checkpoints[jobID]
	if !ok {
		checkpoint = agentMemoryLifecycleCheckpoint{completedTaskIDs: make(map[string]struct{})}
		s.checkpoints[jobID] = checkpoint
		return checkpoint
	}
	if checkpoint.completedTaskIDs == nil {
		checkpoint.completedTaskIDs = make(map[string]struct{})
		s.checkpoints[jobID] = checkpoint
	}
	return checkpoint
}

func cloneAgentMemoryLifecycleCheckpoint(checkpoint agentMemoryLifecycleCheckpoint) agentMemoryLifecycleCheckpoint {
	clonedTaskIDs := make(map[string]struct{}, len(checkpoint.completedTaskIDs))
	for taskID := range checkpoint.completedTaskIDs {
		clonedTaskIDs[taskID] = struct{}{}
	}
	return agentMemoryLifecycleCheckpoint{
		nextIndex:        checkpoint.nextIndex,
		completedTaskIDs: clonedTaskIDs,
		paused:           checkpoint.paused,
		canceled:         checkpoint.canceled,
	}
}

func filterAgentMemoryByLifecycle(items []agentMemoryLifecycleMemoryItem, includeArchived bool) []agentMemoryLifecycleMemoryItem {
	if includeArchived {
		result := make([]agentMemoryLifecycleMemoryItem, len(items))
		copy(result, items)
		return result
	}
	result := make([]agentMemoryLifecycleMemoryItem, 0, len(items))
	for _, item := range items {
		if item.lifecycleState == agentMemoryLifecycleStateArchive {
			continue
		}
		result = append(result, item)
	}
	return result
}
