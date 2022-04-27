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
)

type JobRepo struct {
	sync.RWMutex
	// List of jobs in use.
	jobs []Job
	// Map job id to job.
	jobMap map[JobId]Job
}

type JobMatch struct {
	JobId JobId
	Job   Job
}

func NewJobRepo(jobMap map[JobId]Job) *JobRepo {
	jobs := []Job{}
	for _, job := range jobMap {
		jobs = append(jobs, job)
	}

	return &JobRepo{
		jobs:   jobs,
		jobMap: jobMap,
	}
}

func (tr *JobRepo) Get(jobId JobId) (job Job, exists bool) {
	tr.RLock()
	defer tr.RUnlock()

	job, exists = tr.jobMap[jobId]
	return
}

func (tr *JobRepo) Add(jobId JobId, job Job) {
	tr.Lock()
	defer tr.Unlock()

	tr.jobMap[jobId] = job
}

// Query jobs.
func (tr *JobRepo) Query(ctx context.Context, pagination *PaginationArgs) <-chan JobMatch {
	if pagination == nil {
		pagination = defaultPagination
	} else if pagination.Limit == 0 {
		pagination.Limit = defaultPagination.Limit
	}

	jobIds := make([]JobId, len(tr.jobMap))
	var idx int
	for jobId, _ := range tr.jobMap {
		jobIds[idx] = jobId
		idx++
	}
	sort.SliceStable(jobIds, func(i, j int) bool {
		return strings.Compare(string(jobIds[i]), (string(jobIds[j]))) < 0
	})

	out := make(chan JobMatch)
	numJobs := int64(len(jobIds))
	lastIdx := max(numJobs-1, 0)
	startIdx := min(pagination.Offset, numJobs)
	endIdx := min(startIdx+pagination.Limit, numJobs)

	// Query.
	go func() {
		defer close(out)

		if startIdx > lastIdx {
			return
		}

		for _, jobId := range jobIds[startIdx:endIdx] {
			match := JobMatch{JobId: jobId, Job: tr.jobMap[jobId]}

			select {
			case out <- match:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out
}
