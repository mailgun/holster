package steve

// NewJobHeader creates a JobHeader from Job object.
func NewJobHeader(jobId JobId) *JobHeader {
	return &JobHeader{
		JobId: string(jobId),
	}
}
