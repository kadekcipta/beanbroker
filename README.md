Go Beanstalkd Jobs Broker
=========================

Go API implementation for simple registering *worker* and posting *jobs* to beanstalkd

### Simple usage

```go
package main

import (
	"fmt"
	"strings"
	"time"

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

	if strings.HasSuffix(string(j.Data), "wait") {
		// get the reference to the broker
		b := c.Value(beanbroker.BrokerKey).(beanbroker.JobBroker)
		// use it to post new data
		b.PostJob(&beanbroker.JobRequest{
			Type:  "catcher",
			Data:  []byte("beanstalkd !"),
			Delay: time.Second * 5,
		})
	}
	return beanbroker.Delete
}

func catcher(c context.Context, j *beanbroker.Job) beanbroker.JobResult {
	fmt.Println("Catcher:", string(j.Data))
	return beanbroker.Delete
}

func main() {
	// create root context
	c, cancel := context.WithCancel(context.Background())
	// create connection
	broker := beanbroker.New(c, "localhost:11300")

	// register some collaborative workers
	broker.RegisterWorker(&echoWorker{"Echoer"}, "echo", time.Minute)
	broker.RegisterWorker(beanbroker.WorkerFunc(catcher), "catcher", time.Second*10)

	// post a job
	broker.PostJob(&beanbroker.JobRequest{
		Type: "echo",
		Data: []byte("hello..wait"),
	})

	// wait for enter key
	fmt.Scanln()
	// close root context and propagate
	cancel()
}

```

### Output

```sh
Echoer hello..wait
Catcher: beanstalkd !

```
### NOTES:
Experimental and need many improvements
