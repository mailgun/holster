package main

import (
	"fmt"
	"os"

	"os/signal"
	"syscall"

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

	/*resp, err := client.Get(context.Background(), "/elections/cli-election", clientv3.WithPrefix())
	if err != nil {
		fmt.Printf("while creating a new etcd client: %s\n", err)
		os.Exit(1)
	}*/

	e := etcdutil.NewElection(client, etcdutil.ElectionConfig{
		Election:                "cli-election",
		Candidate:               os.Args[1],
		LeaderChannelSize:       10,
		ResumeLeaderOnReconnect: true,
		TTL: 10,
	})

	err = e.Start()
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

	for leader := range e.LeaderChan() {
		fmt.Printf("[%s] Leader: %t\n", os.Args[1], leader)
	}

}
