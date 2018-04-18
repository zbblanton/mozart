package main

import (
	"log"
	"os"
	"fmt"
	"crypto/tls"
	"crypto/x509"
	"crypto/rand"
	"io/ioutil"
	"net/http"
	"encoding/json"
	"encoding/base64"
	//"flag"
	"gopkg.in/urfave/cli.v1"
)

type Config struct {
  Name string
  ServerIp string
  ServerPort string
  AgentPort string
  AgentJoinKey string
  CaCert string
  CaKey string
  ServerCert string
  ServerKey string
}

var defaultSSLPath = "/etc/mozart/ssl/"
var defaultConfigPath = "/etc/mozart/"

//taken from a google help pack
//https://groups.google.com/forum/#!topic/golang-nuts/rmKTsGHPjlA
func writeFile(file string, config Config){
  f, err := os.Create(file)
  if err != nil {
    panic("cant open file")
  }
  defer f.Close()

  enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
  err = enc.Encode(config)
  if err != nil {
    panic("cant encode")
  }
}

func readFile(file string, config Config) {
  if _, err := os.Stat(file); os.IsNotExist(err) {
    f, err := os.OpenFile(file, os.O_CREATE|os.O_RDONLY, 0644)
    if err != nil {
      panic("cant create file")
    }
    defer f.Close()
  } else {
    f, err := os.OpenFile(file, os.O_CREATE|os.O_RDONLY, 0644)
    if err != nil {
      panic("cant open file")
    }
    defer f.Close()

    enc := json.NewDecoder(f)
    err = enc.Decode(&config)
    if err != nil {
      panic("cant decode")
    }
  }
}

func clusterSwitch(c *cli.Context) {
	fmt.Println("Feature not yet implemented.")
}

func clusterCreate(c *cli.Context) {
	name := c.String("name")
	server := c.String("server")
	if(name == ""){
		log.Fatal("Please provide a name for the server.")
	}

	if(server == ""){
		log.Fatal("Please provide the Mozart server address.")
	}

	fmt.Println("Creating Mozart CA...")
  generateCaKeyPair(name + "-ca")
  fmt.Println("Creating server keypair...")
	generateSignedKeyPair(name + "-ca.crt", name + "-ca.key", name + "-server", server)
  fmt.Println("Creating client keypair...")
	generateSignedKeyPair(name + "-ca.crt", name + "-ca.key", name + "-client", server)

	//Generate worker join key
	randKey := make([]byte, 128)
  _, err := rand.Read(randKey)
  if err != nil {
    fmt.Println("Error generating a new worker key, we are going to exit here due to possible system errors.")
    os.Exit(1)
  }
  joinKey := base64.URLEncoding.EncodeToString(randKey)

	//Create config file
	config := Config{
	  Name: name,
	  ServerIp: server,
	  ServerPort: "8181",
	  AgentPort: "8080",
	  AgentJoinKey: joinKey,
	  CaCert: defaultSSLPath + name + "-ca.crt",
	  CaKey: defaultSSLPath + name + "-ca.key",
	  ServerCert: defaultSSLPath + name + "-server.crt",
	  ServerKey: defaultSSLPath + name + "-server.key",
	}
	writeFile(defaultConfigPath + name + "-config.json", config)


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
