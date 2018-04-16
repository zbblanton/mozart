package main

import (
	"gopkg.in/urfave/cli.v1"
)

var (
	flagClusterName = cli.StringFlag{
		Name:   "name",
		Usage:  "Name of the cluster",
	}

  flagClusterServer = cli.StringFlag{
		Name:   "server",
		Usage:  "Address of the Mozart server.",
	}
)
