package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/mailgun/holster/etcdutil"
	"github.com/sirupsen/logrus"
)

/*func checkErr(err error) {
	if err != nil {
		fmt.Printf("err: %s\n", err)
		os.Exit(1)
	}
}*/

func main() {
	logrus.SetLevel(logrus.DebugLevel)

	if len(os.Args) < 2 {
		fmt.Println("a candidate name is required")
		os.Exit(1)
	}

	client, err := etcdutil.NewClient(nil)
	if err != nil {
		fmt.Printf("while creating a new etcd client: %s\n", err)
		os.Exit(1)
	}

	/*resp, err := client.Get(context.Background(), "/elections/cli-election", clientv3.WithPrefix())
	if err != nil {
		fmt.Printf("while creating a new etcd client: %s\n", err)
		os.Exit(1)
	}*/
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	leaderChan := make(chan etcdutil.Event, 5)
	e, err := etcdutil.NewElection(ctx, client, etcdutil.ElectionConfig{
		Election:  "cli-election",
		Candidate: os.Args[1],
		EventObserver: func(e etcdutil.Event) {
			leaderChan <- e
		},
		TTL: 5,
	})
	if err != nil {
		fmt.Printf("during election start: %s\n", err)
		os.Exit(1)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT)
	go func() {
		for {
			select {
			case sig := <-c:
				switch sig {
				case syscall.SIGINT:
					fmt.Printf("[%s] Concede and exit\n", os.Args[1])
					e.Close()
					os.Exit(1)
				}
			}
		}
	}()

	for e := range leaderChan {
		spew.Printf("[%s] %v\n", os.Args[1], e)
	}
}
