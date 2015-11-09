package beanbroker

import (
	"io"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/kr/beanstalk"
	"golang.org/x/net/context"
)

const (
	maxReconnectAttempt = 50

	BrokerKey = "_beanbroker_"
)

// possibleNetworkError verify the error for network issues
// TODO: need more improvements
func possibleNetworkError(err error) bool {
	return err == io.EOF || err == io.ErrUnexpectedEOF
}

// mustCreateConnection ensures a connection will be created upon return
// passed context will also determine the attempts
func mustCreateConnection(c context.Context, address string) *beanstalk.Conn {
	attempt := 0
	for {
		if attempt > maxReconnectAttempt {
			return nil
		}
		select {
		case <-c.Done():
			return nil
		default:
			conn, err := beanstalk.Dial("tcp", address)
			if err != nil {
				log.Println("beanbroker:", err)
				<-time.After(time.Second * 3)
				continue
			}
			return conn
		}
		attempt++
	}

	panic("unreachable")
}

// beanWorkerHandler implements the beanstalkd worker handling
// it will reserve a job and pass it to interest registered worker
type beanWorkerHandler struct {
	conn               *beanstalk.Conn
	c                  context.Context
	address            string
	w                  Worker
	broker             JobBroker
	jobType            JobType
	reservationTimeout time.Duration
}

// pushJobs push the reserved job to worker
func (p *beanWorkerHandler) pushJobs(w Worker) {
	// create tubeset for topic
	p.conn = mustCreateConnection(p.c, p.address)
	tubeset := beanstalk.NewTubeSet(p.conn, string(p.jobType))

	// create value context to store the reference to broker itself
	ctx := context.WithValue(p.c, BrokerKey, p.broker)

	for {
		select {
		// watch for close signal
		case <-p.c.Done():
			return
		default:
			// get the job
			id, body, err := tubeset.Reserve(p.reservationTimeout)

			// if everything is fine
			if err == nil {
				// pass it to a worker and evaluate the response value
				switch w.Do(ctx, &Job{id, body}) {
				case Delete:
					p.conn.Delete(id)
				case Bury:
					// gets the current priority of the task
					stats, err := p.conn.StatsJob(id)
					if err != nil {
						log.Println("worker:", err.Error())
						continue
					}
					pri, err := strconv.Atoi(stats["pri"])
					if err != nil {
						log.Println("worker:", err.Error())
						continue
					}
					// bury it with its original priority
					p.conn.Bury(id, uint32(pri))
				case Touch:
					p.conn.Touch(id)
				case Release:
					// supports only immediate release
					p.conn.Release(id, 1, time.Second)
				}
				continue
			}

			if err.(beanstalk.ConnError).Err == beanstalk.ErrTimeout {
				continue
			} else if err.(beanstalk.ConnError).Err == beanstalk.ErrDeadline {
				time.Sleep(time.Second)
				continue
			} else if possibleNetworkError(err.(beanstalk.ConnError).Err) {
				// try reconnecting
				p.conn = mustCreateConnection(p.c, p.address)
				tubeset = beanstalk.NewTubeSet(p.conn, string(p.jobType))
			} else {
				log.Println("beanbroker:", err)
			}
		}
	}
}

// beanJobPoster implements beanstalkd job posting
type beanJobPoster struct {
	sync.RWMutex
	conn    *beanstalk.Conn
	c       context.Context
	address string
}

// post put a job in the tube specified by job's type
func (p *beanJobPoster) post(j *JobRequest) error {
	p.Lock()
	defer p.Unlock()

	if p.conn == nil {
		p.conn = mustCreateConnection(p.c, p.address)
	}

	tube := &beanstalk.Tube{p.conn, string(j.Type)}

	_, err := tube.Put(j.Data, j.Priority, j.Delay, j.TTR)
	if err != nil {
		if possibleNetworkError(err.(beanstalk.ConnError).Err) {
			// invalidate to reset state for next call
			p.conn = nil
		}
		return err
	}

	return nil
}

// registerWorker registers a worker including the JobBroker reference
func registerWorker(
	c context.Context,
	broker JobBroker,
	address string,
	w Worker,
	jobType JobType,
	reservationTimeout time.Duration,
) {
	wh := &beanWorkerHandler{
		c:                  c,
		address:            address,
		broker:             broker,
		jobType:            jobType,
		reservationTimeout: reservationTimeout,
	}

	go wh.pushJobs(w)
}

// beanBroker implements the JobBroker interface
type beanBroker struct {
	c         context.Context
	address   string
	jobPoster *beanJobPoster
}

func (b *beanBroker) RegisterWorker(
	w Worker,
	jobType JobType,
	reservationTimeout time.Duration,
) {
	registerWorker(b.c, b, b.address, w, jobType, reservationTimeout)
}

func (b *beanBroker) PostJob(j *JobRequest) error {
	return b.jobPoster.post(j)
}

// factory function
func New(c context.Context, address string) JobBroker {
	jp := &beanJobPoster{
		c:       c,
		address: address,
	}

	return &beanBroker{c, address, jp}
}
