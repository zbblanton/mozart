package main

import (
	"log"
	"os"
	"fmt"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
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

	fmt.Println("Creating Mozart CA...")
  generateCaKeyPair("ca")
  fmt.Println("Creating mozart-server keypair...")
  generateSignedServerKeyPair()
  fmt.Println("Creating client keypair...")
  generateSignedClientKeyPair()

	fmt.Println("Creating the", c.String("name"),"cluster for the Mozart server on", c.String("server") + ".")
}

func clusterList(c *cli.Context) {
	localCaFile := "ca.crt"
	localCertFile := "mozart-client.crt"
	localKeyFile := "mozart-client.key"

	clientCert, err := tls.LoadX509KeyPair(localCertFile, localKeyFile)
	if err != nil {
		panic(err)
	}

	// Get the SystemCertPool, continue with an empty pool on error
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	// Read in the cert file
	certs, err := ioutil.ReadFile(localCaFile)
	if err != nil {
		log.Fatalf("Failed to append %q to RootCAs: %v", localCertFile, err)
	}

	// Append our cert to the system pool
	if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
		log.Println("No certs appended, using system certs only")
	}

	// Trust the augmented cert pool in our client
	config := &tls.Config{
		InsecureSkipVerify: false,
		RootCAs:            rootCAs,
		Certificates: 			[]tls.Certificate{clientCert},
	}
	tr := &http.Transport{TLSClientConfig: config}
	client := &http.Client{Transport: tr}

	// Still works with host-trusted CAs!
	req, err := http.NewRequest(http.MethodGet, "https://10.0.0.28:8181/", nil)
	if err != nil {
		panic(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	bodyStr := string(body)
	fmt.Printf(bodyStr)
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
