package main

import (
	"gopkg.in/urfave/cli.v1"
)

var (
	flagClusterName = cli.StringFlag{
		Name:  "name",
		Usage: "Name of the cluster",
	}

	flagClusterServer = cli.StringFlag{
		Name:  "server",
		Usage: "Address of the Mozart server.",
	}

	flagClusterServers = cli.StringFlag{
		Name:  "servers",
		Usage: "Addresses of each Mozart server separted by a comma. (Multi-Master mode).",
	}

	flagRunConfig = cli.StringFlag{
		Name:  "config",
		Usage: "Container config file.",
	}

	flagAccountName = cli.StringFlag{
		Name:  "account",
		Usage: "Account name.",
	}
)
