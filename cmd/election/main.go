package main

import (
	"fmt"
	"os"
	"time"

	"github.com/mailgun/holster/election"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Printf("a candidate name is required")
		os.Exit(1)
	}

	e, err := election.New(election.Config{
		Election:  "cli-election",
		Candidate: os.Args[1],
	})

	if err != nil {
		fmt.Printf("while creating a new election: %s\n", err)
		os.Exit(1)
	}

	e.Start()
	if err != nil {
		fmt.Printf("during election start: %s\n", err)
		os.Exit(1)
	}

	for {
		fmt.Printf("is '%s' leader? %t", os.Args[1], e.IsLeader())
		time.Sleep(time.Second)
	}
}
