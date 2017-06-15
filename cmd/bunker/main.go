// Bunkercmd is a command-line wrapper for bunker.
package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/codegangsta/cli"
	"github.com/mailgun/cfg"
	"github.com/mailgun/holster/bunker"
)

type Config struct {
	Clusters []bunker.ClusterConfig
	KeyPath  string
}

func initBunker(c *cli.Context) error {
	confPath := c.GlobalString("conf")

	conf := Config{}
	if err := cfg.LoadConfig(confPath, &conf); err != nil {
		fmt.Errorf(err.Error())
		return err
	}

	hmacKey, err := ioutil.ReadFile(conf.KeyPath)
	if err != nil {
		fmt.Errorf(err.Error())
		return err
	}

	if err := bunker.Init(conf.Clusters, bytes.TrimSpace(hmacKey)); err != nil {
		fmt.Errorf(err.Error())
		return err
	}

	return nil
}

func put(c *cli.Context) {
	message := c.Args().First()

	var key string
	var err error

	ttl := c.Duration("ttl")
	if ttl == 0 {
		key, err = bunker.Put(message)
	} else {
		key, err = bunker.PutWithOptions(message, bunker.PutOptions{TTL: ttl})
	}

	if err != nil {
		fmt.Println("error:", err)
	} else {
		fmt.Println("key:", key)
	}
}

func get(c *cli.Context) {
	key := c.Args().First()

	if message, err := bunker.Get(key); err != nil {
		fmt.Println("error:", err)
	} else {
		fmt.Println("message:", message)
	}
}

func delete(c *cli.Context) {
	key := c.Args().First()

	if err := bunker.Delete(key); err != nil {
		fmt.Println("error:", err)
	} else {
		fmt.Println("deleted")
	}
}

func main() {
	app := cli.NewApp()

	app.Name = "bunkercmd"
	app.Usage = "command-line wrapper for bunker library"

	app.Authors = []cli.Author{
		{
			Name:  "Mailgun",
			Email: "admin@mailgunhq.com",
		},
	}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "conf",
			Value: "conf.yaml",
			Usage: "path to a bunker conf file",
		},
	}

	app.Before = initBunker

	app.Commands = []cli.Command{
		{
			Name:  "put",
			Usage: "Saves provided message into bunker",
			Flags: []cli.Flag{
				cli.DurationFlag{
					Name:  "ttl",
					Usage: "optional time-to-live",
				},
			},
			Action: put,
		},
		{
			Name:   "get",
			Usage:  "Retrieves message previously saved into bunker by key",
			Action: get,
		},
		{
			Name:   "delete",
			Usage:  "Deletes message previously saved into bunker by key",
			Action: delete,
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Errorf(err.Error())
	}
}
