package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mailgun/holster/etcdutil"
	"github.com/sirupsen/logrus"
)

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

	e, err := etcdutil.NewElection("cli-election", os.Args[1], client)
	if err != nil {
		fmt.Printf("while creating a new election: %s\n", err)
		os.Exit(1)
	}

	e.Start()
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
					e.Stop()
					os.Exit(1)
				}
			}
		}
	}()

	for {
		fmt.Printf("[%s] Leader: %t\n", os.Args[1], e.IsLeader())
		time.Sleep(time.Second)
	}
}
