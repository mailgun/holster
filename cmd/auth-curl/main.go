package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/mailgun/holster/httpsign"
)

var status = flag.Bool("status", false, "Print the HTTP status code.")

func main() {
	flag.Parse()
	if flag.NArg() != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <keypath> <URL>\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	_, err := os.Stat(flag.Arg(0))
	checkErr(err)

	svc, err := httpsign.New(&httpsign.Config{
		KeyPath:        flag.Arg(0),
		SignVerbAndURI: true,
	})
	checkErr(err)

	req, err := http.NewRequest("GET", flag.Arg(1), nil)
	checkErr(err)

	err = svc.SignRequest(req)
	checkErr(err)

	resp, err := http.DefaultClient.Do(req)
	checkErr(err)

	defer resp.Body.Close()
	if *status {
		fmt.Println(resp.Status)
	}
	io.Copy(os.Stdout, resp.Body)
}

func checkErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
