package main

import (
	"fmt"

	"github.com/kadekcipta/beanbroker"
	"golang.org/x/net/context"
)

// simple smart echo worker
type echoWorker struct {
	id string
}

func (e *echoWorker) Do(c context.Context, j *beanbroker.Job) beanbroker.JobResult {
	// print the data as string
	fmt.Println(e.id, string(j.Data))

	if string(j.Data) == "hello" {
		// get the reference to the broker
		b := c.Value(beanbroker.BrokerKey).(beanbroker.JobBroker)
		// use it to post new data
		b.PostJob(&beanbroker.JobRequest{
			Type: "echo",
			Data: []byte("beanstalkd !"),
		})
	}
	return beanbroker.Delete
}

func (e *echoWorker) Interest() beanbroker.JobType {
	return "echo"
}

func main() {
	// create root context
	c, cancel := context.WithCancel(context.Background())
	// create connection
	broker := beanbroker.New(c, "127.0.0.1:11300")

	// register some collaborative workers
	broker.RegisterWorker(&echoWorker{"Worker #1"})
	broker.RegisterWorker(&echoWorker{"Worker #2"})

	// post a job
	broker.PostJob(&beanbroker.JobRequest{
		Type: "echo",
		Data: []byte("hello"),
	})

	// wait for enter key
	fmt.Scanln()
	// close root context and propagate
	cancel()
}
