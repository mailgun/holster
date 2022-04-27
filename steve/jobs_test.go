package steve_test

import (
	"context"
	"fmt"
	"io"
	"sort"
	"sync"
	"testing"

	"github.com/mailgun/holster/v4/errors"
	"github.com/mailgun/holster/v4/steve"
	"github.com/mailgun/holster/v4/syncutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type JobTaskItem struct {
	JobId  steve.JobId
	Job    *MockJob
	TaskId steve.TaskId
}

func TestJobs(t *testing.T) {
	const grpcPort = 0
	const actionName = "Mock action"
	ctx := context.Background()

	const outputLines = 25
	taskOutput := make([]string, outputLines)
	for idx := 0; idx < outputLines; idx++ {
		taskOutput[idx] = fmt.Sprintf("Line %d", idx+1)
	}

	t.Run("HealthCheck()", func(t *testing.T) {
		// Start gRPC server.
		testSrv, err := NewTestServer(grpcPort, map[steve.JobId]steve.Job{})
		require.NoError(t, err)
		client := testSrv.jobClt
		defer func() {
			err = testSrv.Close()
			require.NoError(t, err)
		}()

		// Call code.
		resp, err := client.HealthCheck(ctx, &emptypb.Empty{})

		// Verify.
		require.NoError(t, err)
		assert.Equal(t, "OK", resp.Message)
	})

	t.Run("GetJobs()", func(t *testing.T) {
		// Start gRPC server.
		testSrv, err := NewTestServer(grpcPort, fakeJobMap)
		require.NoError(t, err)
		client := testSrv.jobClt
		defer func() {
			err = testSrv.Close()
			require.NoError(t, err)
		}()

		t.Run("No arguments", func(t *testing.T) {
			// Call code.
			resp, err := client.GetJobs(ctx, &steve.GetJobsReq{})

			// Verify.
			require.NoError(t, err)

			// Expect first 20 actions verbatim.
			require.Len(t, resp.Jobs, 20)
			for idx, job := range resp.Jobs {
				jobId := steve.JobId(job.JobId)
				expectedJobId := fakeJobs[idx].(*FakeJob).JobId
				assert.Equal(t, expectedJobId, jobId)
			}
		})

		t.Run("Pagination", func(t *testing.T) {
			t.Run("Offset", func(t *testing.T) {
				// Call code.
				resp, err := client.GetJobs(ctx, &steve.GetJobsReq{
					Pagination: &steve.PaginationArgs{Offset: 10},
				})

				// Verify.
				require.NoError(t, err)

				require.Len(t, resp.Jobs, 20)
				for idx, job := range resp.Jobs {
					jobId := steve.JobId(job.JobId)
					expectedJobId := fakeJobs[idx+10].(*FakeJob).JobId
					assert.Equal(t, expectedJobId, jobId)
				}
			})

			t.Run("Limit", func(t *testing.T) {
				// Call code.
				resp, err := client.GetJobs(ctx, &steve.GetJobsReq{
					Pagination: &steve.PaginationArgs{Limit: 5},
				})

				// Verify.
				require.NoError(t, err)

				require.Len(t, resp.Jobs, 5)
				for idx, job := range resp.Jobs {
					jobId := steve.JobId(job.JobId)
					expectedJobId := fakeJobs[idx].(*FakeJob).JobId
					assert.Equal(t, expectedJobId, jobId)
				}
			})

			t.Run("Offset and limit", func(t *testing.T) {
				// Call code.
				resp, err := client.GetJobs(ctx, &steve.GetJobsReq{
					Pagination: &steve.PaginationArgs{Offset: 10, Limit: 5},
				})

				// Verify.
				require.NoError(t, err)

				require.Len(t, resp.Jobs, 5)
				for idx, job := range resp.Jobs {
					jobId := steve.JobId(job.JobId)
					expectedJobId := fakeJobs[idx+10].(*FakeJob).JobId
					assert.Equal(t, expectedJobId, jobId)
				}
			})

			t.Run("All values", func(t *testing.T) {
				// Call code.
				resp, err := client.GetJobs(ctx, &steve.GetJobsReq{
					Pagination: &steve.PaginationArgs{Limit: 1000},
				})

				// Verify.
				require.NoError(t, err)

				require.Len(t, resp.Jobs, len(fakeJobs))
				for idx, job := range resp.Jobs {
					jobId := steve.JobId(job.JobId)
					expectedJobId := fakeJobs[idx].(*FakeJob).JobId
					assert.Equal(t, expectedJobId, jobId)
				}
			})
		})
	})

	t.Run("StartTask()", func(t *testing.T) {
		t.Run("Happy path", func(t *testing.T) {
			jobId := steve.JobId("mockJob")

			mockJob := &MockJob{}
			mockJob.On("Start", mock.Anything, mock.Anything, mock.Anything).Once().Return(nil)
			// Status() called once on start.
			mockJob.On("Status", mock.Anything).Once().Return()

			// Start gRPC server.
			jobMap := map[steve.JobId]steve.Job{
				jobId: mockJob,
			}
			testSrv, err := NewTestServer(grpcPort, jobMap)
			require.NoError(t, err)
			client := testSrv.jobClt
			defer func() {
				err = testSrv.Close()
				require.NoError(t, err)
			}()

			// Call code.
			resp, err := client.StartTask(ctx, &steve.StartTaskReq{
				JobId: string(jobId),
			})

			// Verify.
			require.NoError(t, err)
			taskId := steve.TaskId(resp.TaskId)
			assert.NotEmpty(t, taskId)

			mockJob.AssertExpectations(t)

			t.Run("StopTask()", func(t *testing.T) {
				mockJob.On("Stop", mock.Anything).Once().Return(nil)
				// Status() called again on stop.
				mockJob.On("Status", mock.Anything).Once().Return()

				// Call code.
				resp, err := client.StopTask(ctx, &steve.StopTaskReq{
					TaskId: string(taskId),
				})

				// Verify.
				require.NoError(t, err)
				require.NotNil(t, resp)

				mockJob.AssertExpectations(t)
			})
		})

		t.Run("Action not found", func(t *testing.T) {
			// Start gRPC server.
			emptyJobMap := map[steve.JobId]steve.Job{}
			testSrv, err := NewTestServer(grpcPort, emptyJobMap)
			require.NoError(t, err)
			client := testSrv.jobClt
			defer func() {
				err = testSrv.Close()
				require.NoError(t, err)
			}()

			// Call code.
			_, err = client.StartTask(ctx, &steve.StartTaskReq{
				JobId: "bogus",
			})

			// Verify.
			require.Error(t, err)
			errStatus, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, steve.ErrJobNotFound.Error(), errStatus.Message())
		})
	})

	t.Run("GetRunningTasks()", func(t *testing.T) {
		t.Run("Happy path", func(t *testing.T) {
			// Start task.
			jobId := steve.JobId("mockJob")

			mockJob := &MockJob{}
			mockJob.On("Start", mock.Anything, mock.Anything, mock.Anything).Once().Return(nil)
			mockJob.On("Stop", mock.Anything).Once().Return(nil)
			mockJob.On("Status", mock.Anything).Return()

			// Start gRPC server.
			jobMap := map[steve.JobId]steve.Job{
				jobId: mockJob,
			}
			testSrv, err := NewTestServer(grpcPort, jobMap)
			require.NoError(t, err)
			client := testSrv.jobClt
			defer func() {
				err = testSrv.Close()
				require.NoError(t, err)
			}()

			startResp, err := client.StartTask(ctx, &steve.StartTaskReq{
				JobId: string(jobId),
			})
			require.NoError(t, err)
			taskId := steve.TaskId(startResp.TaskId)

			// Call code.
			getResp, err := client.GetRunningTasks(ctx, &steve.GetRunningTasksReq{
				Filter: &steve.TaskFilter{
					TaskIds: []string{string(taskId)},
				},
			})

			// Verify.
			require.NoError(t, err)
			require.Len(t, getResp.Tasks, 1)
			jobTask := JobTaskItem{JobId: jobId, Job: mockJob, TaskId: taskId}
			AssertEqualGetRunningTask(t, jobTask, getResp.Tasks[0])

			// Clean up.
			_, err = client.StopTask(ctx, &steve.StopTaskReq{
				TaskId: string(taskId),
			})
			require.NoError(t, err)

			mockJob.AssertExpectations(t)
		})

		t.Run("Filter", func(t *testing.T) {
			// Start gRPC server.
			emptyJobMap := map[steve.JobId]steve.Job{}
			testSrv, err := NewTestServer(grpcPort, emptyJobMap)
			require.NoError(t, err)
			client := testSrv.jobClt
			defer func() {
				err = testSrv.Close()
				require.NoError(t, err)
			}()

			const numJobs = 5
			jobTasks := testSrv.CreateJobTasks(ctx, t, numJobs)

			t.Run("by job ids", func(t *testing.T) {
				// Start a bunch of
				// Query first 3 action ids.
				getResp, err := client.GetRunningTasks(ctx, &steve.GetRunningTasksReq{
					Filter: &steve.TaskFilter{
						JobIds: []string{
							string(jobTasks[0].JobId),
							string(jobTasks[1].JobId),
							string(jobTasks[2].JobId),
						},
					},
				})

				// Verify.
				require.NoError(t, err)
				require.Len(t, getResp.Tasks, 3)

				// Returned values are sorted by action id, so sort expectations to match.
				expectedJobTasks := jobTasks[0:3]
				sort.SliceStable(expectedJobTasks, func(i, j int) bool {
					return expectedJobTasks[i].JobId < expectedJobTasks[j].JobId
				})

				for i := 0; i < 3; i++ {
					AssertEqualGetRunningTask(t, expectedJobTasks[i], getResp.Tasks[i])
				}
			})

			t.Run("by task ids", func(t *testing.T) {
				// Start a bunch of
				// Query first 3 action ids.
				getResp, err := client.GetRunningTasks(ctx, &steve.GetRunningTasksReq{
					Filter: &steve.TaskFilter{
						TaskIds: []string{
							string(jobTasks[0].TaskId),
							string(jobTasks[1].TaskId),
							string(jobTasks[2].TaskId),
						},
					},
				})

				// Verify.
				require.NoError(t, err)
				require.Len(t, getResp.Tasks, 3)

				// Returned values are sorted by action id, so sort expectations to match.
				expectedJobTasks := jobTasks[0:3]
				sort.SliceStable(expectedJobTasks, func(i, j int) bool {
					return expectedJobTasks[i].JobId < expectedJobTasks[j].JobId
				})

				for i := 0; i < 3; i++ {
					AssertEqualGetRunningTask(t, expectedJobTasks[i], getResp.Tasks[i])
				}
			})
		})

		t.Run("Pagination", func(t *testing.T) {
			// Start gRPC server.
			emptyJobMap := map[steve.JobId]steve.Job{}
			testSrv, err := NewTestServer(grpcPort, emptyJobMap)
			require.NoError(t, err)
			client := testSrv.jobClt
			defer func() {
				err = testSrv.Close()
				require.NoError(t, err)
			}()

			// Start a bunch of
			const numJobs = 25
			jobTasks := testSrv.CreateJobTasks(ctx, t, numJobs)

			t.Run("Offset", func(t *testing.T) {
				// Call code.
				getResp, err := client.GetRunningTasks(ctx, &steve.GetRunningTasksReq{
					Pagination: &steve.PaginationArgs{Offset: 2},
				})

				// Verify.
				require.NoError(t, err)
				// Expect first 20 matches.
				require.Len(t, getResp.Tasks, 20)

				// Returned values are sorted by action id, so sort expectations to match.
				expectedJobTasks := jobTasks[0:]
				sort.SliceStable(expectedJobTasks, func(i, j int) bool {
					return expectedJobTasks[i].JobId < expectedJobTasks[j].JobId
				})
				expectedJobTasks = expectedJobTasks[2:]

				for i := 0; i < 20; i++ {
					AssertEqualGetRunningTask(t, expectedJobTasks[i], getResp.Tasks[i])
				}
			})

			t.Run("Limit", func(t *testing.T) {
				// Call code.
				getResp, err := client.GetRunningTasks(ctx, &steve.GetRunningTasksReq{
					Pagination: &steve.PaginationArgs{Limit: 5},
				})

				// Verify.
				require.NoError(t, err)
				require.Len(t, getResp.Tasks, 5)

				// Returned values are sorted by action id, so sort expectations to match.
				expectedJobTasks := jobTasks[0:]
				sort.SliceStable(expectedJobTasks, func(i, j int) bool {
					return expectedJobTasks[i].JobId < expectedJobTasks[j].JobId
				})

				for i := 0; i < 5; i++ {
					AssertEqualGetRunningTask(t, expectedJobTasks[i], getResp.Tasks[i])
				}
			})

			t.Run("Offset and limit", func(t *testing.T) {
				// Call code.
				getResp, err := client.GetRunningTasks(ctx, &steve.GetRunningTasksReq{
					Pagination: &steve.PaginationArgs{Offset: 2, Limit: 5},
				})

				// Verify.
				require.NoError(t, err)
				require.Len(t, getResp.Tasks, 5)

				// Returned values are sorted by action id, so sort expectations to match.
				expectedJobTasks := jobTasks[0:]
				sort.SliceStable(expectedJobTasks, func(i, j int) bool {
					return expectedJobTasks[i].JobId < expectedJobTasks[j].JobId
				})
				expectedJobTasks = expectedJobTasks[2:]

				for i := 0; i < 5; i++ {
					AssertEqualGetRunningTask(t, expectedJobTasks[i], getResp.Tasks[i])
				}
			})

			t.Run("All values", func(t *testing.T) {
				// Call code.
				getResp, err := client.GetRunningTasks(ctx, &steve.GetRunningTasksReq{
					Pagination: &steve.PaginationArgs{Limit: 1000},
				})

				// Verify.
				require.NoError(t, err)
				require.Len(t, getResp.Tasks, numJobs)

				// Returned values are sorted by action id, so sort expectations to match.
				expectedJobTasks := jobTasks[0:]
				sort.SliceStable(expectedJobTasks, func(i, j int) bool {
					return expectedJobTasks[i].JobId < expectedJobTasks[j].JobId
				})

				for i := 0; i < numJobs; i++ {
					AssertEqualGetRunningTask(t, expectedJobTasks[i], getResp.Tasks[i])
				}
			})
		})
	})

	t.Run("GetStoppedTasks()", func(t *testing.T) {
		t.Run("Happy path", func(t *testing.T) {
			// Start job.
			jobId := steve.JobId("mockJob")

			mockJob := &MockJob{}
			mockJob.On("Start", mock.Anything, mock.Anything, mock.Anything).Once().Return(nil)
			mockJob.On("Stop", mock.Anything).Once().Return(nil)
			mockJob.On("Status", mock.Anything).Return()

			// Start gRPC server.
			jobMap := map[steve.JobId]steve.Job{
				jobId: mockJob,
			}
			testSrv, err := NewTestServer(grpcPort, jobMap)
			require.NoError(t, err)
			client := testSrv.jobClt
			defer func() {
				err = testSrv.Close()
				require.NoError(t, err)
			}()

			// Start and stop a job.
			startResp, err := client.StartTask(ctx, &steve.StartTaskReq{
				JobId: string(jobId),
			})
			require.NoError(t, err)
			taskId := steve.TaskId(startResp.TaskId)

			_, err = client.StopTask(ctx, &steve.StopTaskReq{
				TaskId: string(taskId),
			})
			require.NoError(t, err)

			// Call code.
			getResp, err := client.GetStoppedTasks(ctx, &steve.GetStoppedTasksReq{
				Filter: &steve.TaskFilter{
					TaskIds: []string{string(taskId)},
				},
			})

			// Verify.
			require.NoError(t, err)
			require.Len(t, getResp.Tasks, 1)
			jobTask := JobTaskItem{JobId: jobId, Job: mockJob, TaskId: taskId}
			AssertEqualGetStoppedTask(t, jobTask, getResp.Tasks[0])

			mockJob.AssertExpectations(t)
		})

		t.Run("Action returns error", func(t *testing.T) {
			// Start job.
			var srvWg sync.WaitGroup
			defer srvWg.Wait()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			jobId := steve.JobId("mockJob")
			mockJob, taskId, _, closer, client := setupMockTask(ctx, t, srvWg)
			closer.Close(errors.New("Foobar error"))

			// Wait for job to stop.
			waitForStop(ctx, t, client, taskId)

			// Call code.
			getResp, err := client.GetStoppedTasks(ctx, &steve.GetStoppedTasksReq{
				Filter: &steve.TaskFilter{
					TaskIds: []string{string(taskId)},
				},
			})

			// Verify.
			require.NoError(t, err)
			require.Len(t, getResp.Tasks, 1)
			jobTask := JobTaskItem{JobId: jobId, Job: mockJob, TaskId: taskId}
			AssertEqualGetStoppedTask(t, jobTask, getResp.Tasks[0])
			assert.Contains(t, "Foobar error", getResp.Tasks[0].Error)

			mockJob.AssertExpectations(t)
		})

		t.Run("Filter", func(t *testing.T) {
			// Start gRPC server.
			emptyJobMap := map[steve.JobId]steve.Job{}
			testSrv, err := NewTestServer(grpcPort, emptyJobMap)
			require.NoError(t, err)
			client := testSrv.jobClt
			defer func() {
				err = testSrv.Close()
				require.NoError(t, err)
			}()

			const numJobs = 5
			jobTasks := testSrv.CreateJobTasks(ctx, t, numJobs)

			for _, jobTask := range jobTasks {
				_, err := client.StopTask(ctx, &steve.StopTaskReq{
					TaskId: string(jobTask.TaskId),
				})
				require.NoError(t, err)
				jobTask.Job.AssertExpectations(t)
			}

			t.Run("by job ids", func(t *testing.T) {
				// Start and stop a bunch of
				// Query first 3 action ids.
				// Call code.
				getResp, err := client.GetStoppedTasks(ctx, &steve.GetStoppedTasksReq{
					Filter: &steve.TaskFilter{
						JobIds: []string{
							string(jobTasks[0].JobId),
							string(jobTasks[1].JobId),
							string(jobTasks[2].JobId),
						},
					},
				})

				// Verify.
				require.NoError(t, err)
				require.Len(t, getResp.Tasks, 3)

				// Returned values are sorted by action id, so sort expectations to match.
				expectedJobTasks := jobTasks[0:3]
				sort.SliceStable(expectedJobTasks, func(i, j int) bool {
					return expectedJobTasks[i].JobId < expectedJobTasks[j].JobId
				})

				for i := 0; i < 3; i++ {
					AssertEqualGetStoppedTask(t, expectedJobTasks[i], getResp.Tasks[i])
				}
			})

			t.Run("by task ids", func(t *testing.T) {
				// Start and stop a bunch of
				// Query first 3 action ids.
				// Call code.
				getResp, err := client.GetStoppedTasks(ctx, &steve.GetStoppedTasksReq{
					Filter: &steve.TaskFilter{
						TaskIds: []string{
							string(jobTasks[0].TaskId),
							string(jobTasks[1].TaskId),
							string(jobTasks[2].TaskId),
						},
					},
				})

				// Verify.
				require.NoError(t, err)
				require.Len(t, getResp.Tasks, 3)

				// Returned values are sorted by action id, so sort expectations to match.
				expectedJobTasks := jobTasks[0:3]
				sort.SliceStable(expectedJobTasks, func(i, j int) bool {
					return expectedJobTasks[i].JobId < expectedJobTasks[j].JobId
				})

				for i := 0; i < 3; i++ {
					AssertEqualGetStoppedTask(t, expectedJobTasks[i], getResp.Tasks[i])
				}
			})
		})

		t.Run("Pagination", func(t *testing.T) {
			// Start gRPC server.
			emptyJobMap := map[steve.JobId]steve.Job{}
			testSrv, err := NewTestServer(grpcPort, emptyJobMap)
			require.NoError(t, err)
			client := testSrv.jobClt
			defer func() {
				err = testSrv.Close()
				require.NoError(t, err)
			}()

			const numJobs = 25
			jobTasks := testSrv.CreateJobTasks(ctx, t, numJobs)

			for _, jobTask := range jobTasks {
				_, err := client.StopTask(ctx, &steve.StopTaskReq{
					TaskId: string(jobTask.TaskId),
				})
				require.NoError(t, err)
				jobTask.Job.AssertExpectations(t)
			}

			t.Run("Offset", func(t *testing.T) {
				// Start and stop a bunch of
				// Call code.
				getResp, err := client.GetStoppedTasks(ctx, &steve.GetStoppedTasksReq{
					Pagination: &steve.PaginationArgs{Offset: 2},
				})

				// Verify.
				require.NoError(t, err)
				// Expect first 20 matches.
				require.Len(t, getResp.Tasks, 20)

				// Returned values are sorted by action id, so sort expectations to match.
				expectedJobTasks := jobTasks[0:]
				sort.SliceStable(expectedJobTasks, func(i, j int) bool {
					return expectedJobTasks[i].JobId < expectedJobTasks[j].JobId
				})
				expectedJobTasks = expectedJobTasks[2:]

				for i := 0; i < 20; i++ {
					AssertEqualGetStoppedTask(t, expectedJobTasks[i], getResp.Tasks[i])
				}
			})

			t.Run("Limit", func(t *testing.T) {
				// Start and stop a bunch of
				// Call code.
				getResp, err := client.GetStoppedTasks(ctx, &steve.GetStoppedTasksReq{
					Pagination: &steve.PaginationArgs{Limit: 5},
				})

				// Verify.
				require.NoError(t, err)
				require.Len(t, getResp.Tasks, 5)

				// Returned values are sorted by action id, so sort expectations to match.
				expectedJobTasks := jobTasks[0:]
				sort.SliceStable(expectedJobTasks, func(i, j int) bool {
					return expectedJobTasks[i].JobId < expectedJobTasks[j].JobId
				})

				for i := 0; i < 5; i++ {
					AssertEqualGetStoppedTask(t, expectedJobTasks[i], getResp.Tasks[i])
				}
			})

			t.Run("Offset and limit", func(t *testing.T) {
				// Start and stop a bunch of
				// Call code.
				getResp, err := client.GetStoppedTasks(ctx, &steve.GetStoppedTasksReq{
					Pagination: &steve.PaginationArgs{Offset: 2, Limit: 5},
				})

				// Verify.
				require.NoError(t, err)
				require.Len(t, getResp.Tasks, 5)

				// Returned values are sorted by action id, so sort expectations to match.
				expectedJobTasks := jobTasks[0:]
				sort.SliceStable(expectedJobTasks, func(i, j int) bool {
					return expectedJobTasks[i].JobId < expectedJobTasks[j].JobId
				})
				expectedJobTasks = expectedJobTasks[2:]

				for i := 0; i < 5; i++ {
					AssertEqualGetStoppedTask(t, expectedJobTasks[i], getResp.Tasks[i])
				}
			})

			t.Run("All values", func(t *testing.T) {
				// Start and stop a bunch of
				// Call code.
				getResp, err := client.GetStoppedTasks(ctx, &steve.GetStoppedTasksReq{
					Pagination: &steve.PaginationArgs{Limit: 1000},
				})

				// Verify.
				require.NoError(t, err)
				require.Len(t, getResp.Tasks, numJobs)

				// Returned values are sorted by action id, so sort expectations to match.
				expectedJobTasks := jobTasks[0:]
				sort.SliceStable(expectedJobTasks, func(i, j int) bool {
					return expectedJobTasks[i].JobId < expectedJobTasks[j].JobId
				})

				for i := 0; i < numJobs; i++ {
					AssertEqualGetStoppedTask(t, expectedJobTasks[i], getResp.Tasks[i])
				}
			})
		})
	})

	t.Run("Get task output", func(t *testing.T) {
		t.Run("While running", func(t *testing.T) {
			var srvWg sync.WaitGroup
			defer srvWg.Wait()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			mockJob, taskId, writer, _, client := setupMockTask(ctx, t, srvWg)

			// Send some output.
			const numLines = 10
			var expectedOutput []string
			var wg syncutil.WaitGroup
			wg.Go(func() {
				for i := 0; i < numLines; i++ {
					line := fmt.Sprintf("Line %d", i+1)
					writer.Write([]byte(line + "\n"))
					expectedOutput = append(expectedOutput, line)
				}
			})

			// Wait for task to finish.
			wg.Wait()

			// Call code.
			outputResp, err := client.GetTaskOutput(ctx, &steve.GetTaskOutputReq{
				TaskId: string(taskId),
			})

			// Verify.
			require.NoError(t, err)
			assert.Equal(t, expectedOutput, outputResp.Output)

			// Clean up.
			_, err = client.StopTask(ctx, &steve.StopTaskReq{
				TaskId: string(taskId),
			})
			require.NoError(t, err)

			mockJob.AssertExpectations(t)
		})

		t.Run("After task finished", func(t *testing.T) {
			var srvWg sync.WaitGroup
			defer srvWg.Wait()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			mockJob, taskId, writer, _, client := setupMockTask(ctx, t, srvWg)

			// Send some output.
			const numLines = 10
			var expectedOutput []string
			var wg syncutil.WaitGroup
			wg.Go(func() {
				for i := 0; i < numLines; i++ {
					line := fmt.Sprintf("Line %d", i+1)
					writer.Write([]byte(line + "\n"))
					expectedOutput = append(expectedOutput, line)
				}
			})

			// Wait for task to finish.
			wg.Wait()

			// Stop job.
			_, err := client.StopTask(ctx, &steve.StopTaskReq{
				TaskId: string(taskId),
			})
			require.NoError(t, err)

			// Call code.
			outputResp, err := client.GetTaskOutput(ctx, &steve.GetTaskOutputReq{
				TaskId: string(taskId),
			})

			// Verify.
			require.NoError(t, err)
			assert.Equal(t, expectedOutput, outputResp.Output)

			mockJob.AssertExpectations(t)
		})

		t.Run("Pagination", func(t *testing.T) {
			t.Run("No arguments", func(t *testing.T) {
				var srvWg sync.WaitGroup
				defer srvWg.Wait()
				ctx, cancel := context.WithCancel(ctx)
				defer cancel()
				mockJob, taskId, writer, _, client := setupMockTask(ctx, t, srvWg)

				// Send some output.
				var expectedOutput []string
				var wg syncutil.WaitGroup
				wg.Go(func() {
					for idx, line := range taskOutput {
						writer.Write([]byte(line + "\n"))

						// Default limit 20.
						if idx < 20 {
							expectedOutput = append(expectedOutput, line)
						}
					}
				})

				// Wait for task to finish.
				wg.Wait()

				// Call code.
				outputResp, err := client.GetTaskOutput(ctx, &steve.GetTaskOutputReq{
					TaskId: string(taskId),
				})

				// Verify.
				require.NoError(t, err)
				assert.Equal(t, expectedOutput, outputResp.Output)

				mockJob.AssertExpectations(t)
			})

			t.Run("Limit", func(t *testing.T) {
				var srvWg sync.WaitGroup
				defer srvWg.Wait()
				ctx, cancel := context.WithCancel(ctx)
				defer cancel()
				mockJob, taskId, writer, _, client := setupMockTask(ctx, t, srvWg)

				// Send some output.
				const limit = 5
				var expectedOutput []string
				var wg syncutil.WaitGroup
				wg.Go(func() {
					for idx, line := range taskOutput {
						writer.Write([]byte(line + "\n"))

						if idx < limit {
							expectedOutput = append(expectedOutput, line)
						}
					}
				})

				// Wait for task to finish.
				wg.Wait()

				// Call code.
				outputResp, err := client.GetTaskOutput(ctx, &steve.GetTaskOutputReq{
					TaskId:     string(taskId),
					Pagination: &steve.PaginationArgs{Limit: limit},
				})

				// Verify.
				require.NoError(t, err)
				assert.Equal(t, expectedOutput, outputResp.Output)

				mockJob.AssertExpectations(t)
			})

			t.Run("Offset", func(t *testing.T) {
				var srvWg sync.WaitGroup
				defer srvWg.Wait()
				ctx, cancel := context.WithCancel(ctx)
				defer cancel()
				mockJob, taskId, writer, _, client := setupMockTask(ctx, t, srvWg)

				// Send some output.
				const offset = 10
				var expectedOutput []string
				var wg syncutil.WaitGroup
				wg.Go(func() {
					for idx, line := range taskOutput {
						writer.Write([]byte(line + "\n"))

						if idx >= offset {
							expectedOutput = append(expectedOutput, line)
						}
					}
				})

				// Wait for task to finish.
				wg.Wait()

				// Call code.
				outputResp, err := client.GetTaskOutput(ctx, &steve.GetTaskOutputReq{
					TaskId:     string(taskId),
					Pagination: &steve.PaginationArgs{Offset: offset},
				})

				// Verify.
				require.NoError(t, err)
				assert.Equal(t, expectedOutput, outputResp.Output)

				mockJob.AssertExpectations(t)
			})

			t.Run("Offset and limit", func(t *testing.T) {
				var srvWg sync.WaitGroup
				defer srvWg.Wait()
				ctx, cancel := context.WithCancel(ctx)
				defer cancel()
				mockJob, taskId, writer, _, client := setupMockTask(ctx, t, srvWg)

				// Send some output.
				const limit = 5
				const offset = 2
				var expectedOutput []string
				var wg syncutil.WaitGroup
				wg.Go(func() {
					for idx, line := range taskOutput {
						writer.Write([]byte(line + "\n"))

						if idx >= offset && idx < (limit+offset) {
							expectedOutput = append(expectedOutput, line)
						}
					}
				})

				// Wait for task to finish.
				wg.Wait()

				// Call code.
				outputResp, err := client.GetTaskOutput(ctx, &steve.GetTaskOutputReq{
					TaskId:     string(taskId),
					Pagination: &steve.PaginationArgs{Limit: limit, Offset: offset},
				})

				// Verify.
				require.NoError(t, err)
				assert.Equal(t, expectedOutput, outputResp.Output)

				mockJob.AssertExpectations(t)
			})
		})
	})

	t.Run("Get streaming task output", func(t *testing.T) {
		t.Run("While running", func(t *testing.T) {
			var srvWg sync.WaitGroup
			defer srvWg.Wait()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			mockJob, taskId, writer, closer, client := setupMockTask(ctx, t, srvWg)

			// Send some output.
			const numLines = 10
			var expectedOutput []string
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer closer.Close(nil)
				wg.Done()
				for i := 0; i < numLines; i++ {
					line := fmt.Sprintf("Line %d", i+1)
					writer.Write([]byte(line + "\n"))
					expectedOutput = append(expectedOutput, line)
				}
			}()

			// Wait for task to start sending data.
			wg.Wait()

			// Call code.
			outputClt, err := client.GetStreamingTaskOutput(ctx, &steve.GetTaskOutputReq{
				TaskId: string(taskId),
			})
			require.NoError(t, err)

			actualOutput := []string{}

			for {
				outputResp, err := outputClt.Recv()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)

				actualOutput = append(actualOutput, outputResp.Output...)
			}

			// Verify.
			assert.Equal(t, expectedOutput, actualOutput)

			mockJob.AssertExpectations(t)
		})

		t.Run("After task finished", func(t *testing.T) {
			var srvWg sync.WaitGroup
			defer srvWg.Wait()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			mockJob, taskId, writer, closer, client := setupMockTask(ctx, t, srvWg)

			// Send some output.
			const numLines = 10
			var expectedOutput []string
			var wg syncutil.WaitGroup
			wg.Go(func() {
				defer closer.Close(nil)
				for i := 0; i < numLines; i++ {
					line := fmt.Sprintf("Line %d", i+1)
					writer.Write([]byte(line + "\n"))
					expectedOutput = append(expectedOutput, line)
				}
			})

			// Wait for task to finish.
			wg.Wait()

			// Call code.
			outputClt, err := client.GetStreamingTaskOutput(ctx, &steve.GetTaskOutputReq{
				TaskId: string(taskId),
			})
			require.NoError(t, err)

			actualOutput := []string{}

			for {
				outputResp, err := outputClt.Recv()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)

				actualOutput = append(actualOutput, outputResp.Output...)
			}

			// Verify.
			assert.Equal(t, expectedOutput, actualOutput)

			mockJob.AssertExpectations(t)
		})

		t.Run("Pagination", func(t *testing.T) {
			const outputLines = 25
			taskOutput := make([]string, outputLines)
			for idx := 0; idx < outputLines; idx++ {
				taskOutput[idx] = fmt.Sprintf("Line %d", idx+1)
			}

			t.Run("No arguments", func(t *testing.T) {
				var srvWg sync.WaitGroup
				defer srvWg.Wait()
				ctx, cancel := context.WithCancel(ctx)
				defer cancel()
				mockJob, taskId, writer, _, client := setupMockTask(ctx, t, srvWg)

				// Send some output.
				var expectedOutput []string
				var wg syncutil.WaitGroup
				wg.Go(func() {
					for _, line := range taskOutput {
						writer.Write([]byte(line + "\n"))
						expectedOutput = append(expectedOutput, line)
					}
				})

				// Wait until task finishes.
				wg.Wait()

				// Stop job.
				_, err := client.StopTask(ctx, &steve.StopTaskReq{
					TaskId: string(taskId),
				})
				require.NoError(t, err)

				// Call code.
				outputClt, err := client.GetStreamingTaskOutput(ctx, &steve.GetTaskOutputReq{
					TaskId: string(taskId),
				})
				require.NoError(t, err)

				actualOutput := []string{}

				for {
					outputResp, err := outputClt.Recv()
					if err == io.EOF {
						break
					}
					require.NoError(t, err)

					actualOutput = append(actualOutput, outputResp.Output...)
				}

				// Verify.
				require.NoError(t, err)
				assert.Equal(t, expectedOutput, actualOutput)

				mockJob.AssertExpectations(t)
			})

			t.Run("Offset", func(t *testing.T) {
				var srvWg sync.WaitGroup
				defer srvWg.Wait()
				ctx, cancel := context.WithCancel(ctx)
				defer cancel()
				mockJob, taskId, writer, _, client := setupMockTask(ctx, t, srvWg)

				// Send some output.
				const offset = 10
				var expectedOutput []string
				var wg syncutil.WaitGroup
				wg.Go(func() {
					for idx, line := range taskOutput {
						writer.Write([]byte(line + "\n"))

						if idx >= offset {
							expectedOutput = append(expectedOutput, line)
						}
					}
				})

				// Wait until task finishes.
				wg.Wait()

				// Stop job.
				_, err := client.StopTask(ctx, &steve.StopTaskReq{
					TaskId: string(taskId),
				})
				require.NoError(t, err)

				// Call code.
				outputClt, err := client.GetStreamingTaskOutput(ctx, &steve.GetTaskOutputReq{
					TaskId:     string(taskId),
					Pagination: &steve.PaginationArgs{Offset: offset},
				})
				require.NoError(t, err)

				actualOutput := []string{}

				for {
					outputResp, err := outputClt.Recv()
					if err == io.EOF {
						break
					}
					require.NoError(t, err)

					actualOutput = append(actualOutput, outputResp.Output...)
				}

				// Verify.
				require.NoError(t, err)
				assert.Equal(t, expectedOutput, actualOutput)

				mockJob.AssertExpectations(t)
			})
		})
	})
}

func AssertEqualGetRunningTask(t *testing.T, expectedJobTask JobTaskItem, taskHeader *steve.RunningTaskHeader) {
	assert.Equal(t, string(expectedJobTask.JobId), taskHeader.Job.JobId)
	assert.Equal(t, string(expectedJobTask.TaskId), taskHeader.TaskId)
	assert.False(t, taskHeader.Started.AsTime().IsZero())
}

func AssertEqualGetStoppedTask(t *testing.T, expectedJobTask JobTaskItem, taskHeader *steve.StoppedTaskHeader) {
	assert.Equal(t, string(expectedJobTask.JobId), taskHeader.Job.JobId)
	assert.Equal(t, string(expectedJobTask.TaskId), taskHeader.TaskId)
	assert.False(t, taskHeader.Started.AsTime().IsZero())
	assert.False(t, taskHeader.Stopped.AsTime().IsZero())
}

// Start a TestServer, create a MockJob, start a task, and capture the writer object.
// Be sure to cancel the context to teardown resources.
// Then use `WaitGroup` to wait for teardown to finish.
func setupMockTask(ctx context.Context, t *testing.T, wg sync.WaitGroup) (*MockJob, steve.TaskId, io.Writer, *steve.TaskCloser, steve.JobsV1Client) {
	const grpcPort = 0
	wg.Add(1)
	jobId := steve.JobId("mockJob")
	startChan := make(chan struct{})

	mockJob := &MockJob{}
	var writer io.Writer
	var closer *steve.TaskCloser
	mockJob.On("Start", mock.Anything, mock.Anything, mock.Anything).Once().
		Run(func(args mock.Arguments) {
			writer = args.Get(1).(io.Writer)
			closer = args.Get(2).(*steve.TaskCloser)
			close(startChan)
		}).
		Return(nil)
	mockJob.On("Stop", mock.Anything).Maybe().Return(nil)
	mockJob.On("Status", mock.Anything).Return()

	// Start gRPC server.
	jobMap := map[steve.JobId]steve.Job{
		jobId: mockJob,
	}
	testSrv, err := NewTestServer(grpcPort, jobMap)
	require.NoError(t, err)
	client := testSrv.jobClt

	go func() {
		// Tear down when context cancels.
		<-ctx.Done()
		err = testSrv.Close()
		require.NoError(t, err)
		wg.Done()
	}()

	// Start a job.
	startResp, err := client.StartTask(ctx, &steve.StartTaskReq{
		JobId: string(jobId),
	})
	require.NoError(t, err)
	taskId := steve.TaskId(startResp.TaskId)
	assert.NotEmpty(t, taskId)

	// Wait for job to start.
	<-startChan

	return mockJob, taskId, writer, closer, client
}
