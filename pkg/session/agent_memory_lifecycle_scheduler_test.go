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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentMemoryLifecycleTaskIdempotenceAcrossRetries(t *testing.T) {
	scheduler := newAgentMemoryLifecycleScheduler()
	tasks := []agentMemoryLifecycleTask{
		{taskID: "t1", memoryID: 1, targetState: agentMemoryLifecycleStateWarm},
		{taskID: "t2", memoryID: 2, targetState: agentMemoryLifecycleStateCold},
	}
	calls := make(map[string]int)
	_, err := scheduler.runJob("job-idempotent", tasks, func(task agentMemoryLifecycleTask) error {
		calls[task.taskID]++
		if task.taskID == "t2" && calls[task.taskID] == 1 {
			return errors.New("transient")
		}
		return nil
	})
	require.Error(t, err)

	_, err = scheduler.runJob("job-idempotent", tasks, func(task agentMemoryLifecycleTask) error {
		calls[task.taskID]++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, calls["t1"])
	require.Equal(t, 2, calls["t2"])
}

func TestAgentMemoryLifecyclePauseResumeCancelConcurrentSafe(t *testing.T) {
	scheduler := newAgentMemoryLifecycleScheduler()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = scheduler.pauseJob("job-control")
		}()
		go func() {
			defer wg.Done()
			_ = scheduler.resumeJob("job-control")
		}()
	}
	wg.Wait()
	require.NoError(t, scheduler.cancelJob("job-control"))

	_, err := scheduler.runJob("job-control", []agentMemoryLifecycleTask{{taskID: "t1", memoryID: 1, targetState: agentMemoryLifecycleStateArchive}}, func(task agentMemoryLifecycleTask) error {
		return nil
	})
	require.ErrorIs(t, err, errAgentMemoryLifecycleJobCanceled)
}

func TestAgentMemoryLifecycleCheckpointRecovery(t *testing.T) {
	scheduler1 := newAgentMemoryLifecycleScheduler()
	tasks := []agentMemoryLifecycleTask{
		{taskID: "t1", memoryID: 1, targetState: agentMemoryLifecycleStateWarm},
		{taskID: "t2", memoryID: 2, targetState: agentMemoryLifecycleStateCold},
		{taskID: "t3", memoryID: 3, targetState: agentMemoryLifecycleStateArchive},
	}
	calls := make(map[string]int)
	_, err := scheduler1.runJob("job-recovery", tasks, func(task agentMemoryLifecycleTask) error {
		calls[task.taskID]++
		if task.taskID == "t2" {
			return errors.New("node-failover")
		}
		return nil
	})
	require.Error(t, err)

	checkpoint, ok := scheduler1.getCheckpoint("job-recovery")
	require.True(t, ok)

	scheduler2 := newAgentMemoryLifecycleScheduler()
	scheduler2.restoreCheckpoint("job-recovery", checkpoint)
	_, err = scheduler2.runJob("job-recovery", tasks, func(task agentMemoryLifecycleTask) error {
		calls[task.taskID]++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, calls["t1"])
	require.Equal(t, 2, calls["t2"])
	require.Equal(t, 1, calls["t3"])
}

func TestAgentMemoryLifecycleFilterArchivedDefault(t *testing.T) {
	items := []agentMemoryLifecycleMemoryItem{
		{memoryID: 1, lifecycleState: agentMemoryLifecycleStateHot},
		{memoryID: 2, lifecycleState: agentMemoryLifecycleStateArchive},
		{memoryID: 3, lifecycleState: agentMemoryLifecycleStateCold},
	}
	filtered := filterAgentMemoryByLifecycle(items, false)
	require.Equal(t, []uint64{1, 3}, lifecycleMemoryIDs(filtered))

	filteredAll := filterAgentMemoryByLifecycle(items, true)
	require.Equal(t, []uint64{1, 2, 3}, lifecycleMemoryIDs(filteredAll))
}

func lifecycleMemoryIDs(items []agentMemoryLifecycleMemoryItem) []uint64 {
	ids := make([]uint64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.memoryID)
	}
	return ids
}
