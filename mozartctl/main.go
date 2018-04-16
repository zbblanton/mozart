package main

import (
	"log"
	"os"
	"fmt"
	//"flag"
	"gopkg.in/urfave/cli.v1"
)

func clusterSwitch(c *cli.Context) {
	fmt.Println("Feature not yet implemented.")
}

func clusterCreate(c *cli.Context) {
	if(c.String("name") == ""){
		log.Fatal("Please provide a name for the server.")
	}

	if(c.String("server") == ""){
		log.Fatal("Please provide the Mozart server address.")
	}
	fmt.Println("Creating the", c.String("name"),"cluster for the Mozart server on", c.String("server") + ".")
}

func clusterList(c *cli.Context) {
	fmt.Println("Feature not yet implemented.")
}

func serviceCreate(c *cli.Context) {
	fmt.Println("Feature not yet implemented.")
}

func serviceStop(c *cli.Context) {
	fmt.Println("Feature not yet implemented.")
}

func serviceList(c *cli.Context) {
	fmt.Println("Feature not yet implemented.")
}

func main() {
	app := cli.NewApp()
	app.Name = "mozartctl"
	app.Usage = "CLI for Mozart clusters."
	app.Version = "0.1.0"
	app.Commands = []cli.Command{
		{
			Name:        "cluster",
			Usage:       "Helper commands for clusters.",
			Subcommands: []cli.Command{
				{
					Name:  "switch",
					Usage: "switch to another cluster",
					Action: clusterSwitch,
				},
				{
					Name:  "create",
					Usage: "Generate a new cluster config and files.",
					Flags: []cli.Flag{flagClusterName, flagClusterServer},
					Action: clusterCreate,
				},
				{
					Name:  "ls",
					Usage: "List all clusters this client can connect to.",
					Action: clusterList,
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
