package beanbroker

import (
	"time"

	"golang.org/x/net/context"
)

const (
	Bury JobResult = iota
	Delete
	Release
	Touch
)

// JobResult represents the expected action done by broker after Do() returns
type JobResult int

// JobId represents beanstalkd job id
type JobId uint64

// JobData represents data bytes
type JobData []byte

// JobType represents tube name in beanstalkd
type JobType string

// Worker provides common interface for worker implementation
type Worker interface {
	// Do performs whatever the worker supposed to do
	// upon finish or intentional break, return value will determine the job state
	Do(context.Context, *Job) JobResult
}

type WorkerFunc func(context.Context, *Job) JobResult

func (f WorkerFunc) Do(c context.Context, job *Job) JobResult {
	return f(c, job)
}

// Job represents the beanstalkd job
type Job struct {
	Id   uint64
	Data JobData
}

// JobRequest represents job request
type JobRequest struct {
	Type     JobType
	Data     JobData
	Priority uint32
	Delay    time.Duration
	TTR      time.Duration
}

// JobBroker represents central contact point for worker and job poster
type JobBroker interface {
	// RegisterWorker registers the worker and a new connection to a tube is created
	RegisterWorker(w Worker, jobType JobType, reservationTimeout time.Duration)

	// PostJob puts a job request to be dispatched to matched workers
	PostJob(*JobRequest) error
}
