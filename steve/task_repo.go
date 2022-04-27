/*
Copyright 2022 Mailgun Technologies Inc

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package steve

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/mailgun/holster/v4/errors"
)

type TaskRepo struct {
	sync.RWMutex
	// Map job id to task ids.
	jobTaskMap map[JobId]map[TaskId]struct{}
	// Map task id to job id.
	taskJobMap map[TaskId]JobId
}

func NewTaskRepo() *TaskRepo {
	return &TaskRepo{
		jobTaskMap: make(map[JobId]map[TaskId]struct{}),
		taskJobMap: make(map[TaskId]JobId),
	}
}

func (jr *TaskRepo) Set(jobId JobId, taskId TaskId) {
	jr.Lock()
	defer jr.Unlock()

	if m, ok := jr.jobTaskMap[jobId]; ok {
		m[taskId] = struct{}{}
	} else {
		jr.jobTaskMap[jobId] = map[TaskId]struct{}{
			taskId: struct{}{},
		}
	}
	jr.taskJobMap[taskId] = jobId
}

func (jr *TaskRepo) Clear(taskId TaskId) error {
	jr.Lock()
	defer jr.Unlock()

	jobId, ok := jr.taskJobMap[taskId]
	if !ok {
		return errors.New("consistency error, taskId not found in taskJobMap")
	}
	delete(jr.taskJobMap, taskId)
	delete(jr.jobTaskMap[jobId], taskId)
	return nil
}

func (jr *TaskRepo) Has(taskId TaskId) bool {
	jr.RLock()
	defer jr.RUnlock()

	_, ok := jr.taskJobMap[taskId]
	return ok
}

func (jr *TaskRepo) GetJobId(taskId TaskId) (jobId JobId, exists bool) {
	jr.RLock()
	defer jr.RUnlock()

	jobId, exists = jr.taskJobMap[taskId]
	return
}

// Query jobs.
func (jr *TaskRepo) Query(
	ctx context.Context, pagination *PaginationArgs,
	filter *TaskFilter,
) <-chan TaskId {
	jr.RLock()
	defer jr.RUnlock()

	if pagination == nil {
		pagination = defaultPagination
	} else if pagination.Limit == 0 {
		pagination.Limit = defaultPagination.Limit
	}

	out := make(chan TaskId)

	// Prepare filters.
	var filterJobIdSet map[JobId]struct{}
	var filterTaskIdSet map[TaskId]struct{}

	if filter == nil {
		filter = &TaskFilter{}
	}
	if len(filter.JobIds) > 0 {
		filterJobIdSet = make(map[JobId]struct{})

		for _, jobId := range filter.JobIds {
			filterJobIdSet[JobId(jobId)] = struct{}{}
		}
	}
	if len(filter.TaskIds) > 0 {
		filterTaskIdSet = make(map[TaskId]struct{})

		for _, taskId := range filter.TaskIds {
			filterTaskIdSet[TaskId(taskId)] = struct{}{}
		}
	}

	// Query.
	go func() {
		defer close(out)
		skip := pagination.Offset
		remaining := pagination.Limit

		// Get sorted list of job ids.
		sortedJobIds := make([]JobId, len(jr.jobTaskMap))
		for jobId, _ := range jr.jobTaskMap {
			if filterJobIdSet != nil {
				if _, ok := filterJobIdSet[jobId]; !ok {
					continue
				}
			}

			sortedJobIds = append(sortedJobIds, jobId)
		}
		sort.Slice(sortedJobIds, func(i, j int) bool {
			return strings.Compare(
				string(sortedJobIds[i]), string(sortedJobIds[j]),
			) < 0
		})

		for _, iterJobId := range sortedJobIds {
			// Get sorted list of job ids.
			sortedTaskIds := []TaskId{}
			for jobId, _ := range jr.jobTaskMap[iterJobId] {
				if filterTaskIdSet != nil {
					if _, ok := filterTaskIdSet[jobId]; !ok {
						continue
					}
				}

				sortedTaskIds = append(sortedTaskIds, jobId)
			}
			sort.SliceStable(sortedTaskIds, func(i, j int) bool {
				return sortedTaskIds[i] < sortedTaskIds[j]
			})

			for _, iterTaskId := range sortedTaskIds {
				if skip > 0 {
					skip--
					continue
				}

				// Found match.
				select {
				case out <- iterTaskId:
				case <-ctx.Done():
					return
				}

				remaining--
				if remaining == 0 {
					return
				}
			}
		}
	}()

	return out
}
