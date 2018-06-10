package main

import (
	"log"
	"os"
	"fmt"
	"crypto/tls"
	"crypto/x509"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"io/ioutil"
	"net/http"
	"encoding/json"
	"encoding/base64"
	//"flag"
	"gopkg.in/urfave/cli.v1"
	"bufio"
	"github.com/olekukonko/tablewriter"
	"net"
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

type Container struct {
	Name string
	State string
	DesiredState string
	Worker string
}

type ContainerListResp struct {
	Containers map[string]Container
	Success bool
	Error string
}

type Worker struct {
  AgentIp string
  AgentPort string
  Status string
}

type WorkerListResp struct {
	Workers map[string]Worker
	Success bool
	Error string
}

type Resp struct {
  Success bool `json:"success"`
  Error string `json:"error"`
}

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

func readConfigFile(file string) {
  f, err := os.Open(file)
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

func callSecuredServer(pubKey, privKey, ca string, method string, url string, body io.Reader) (respBody []byte, err error)  {
  //Load our key pair
  clientKeyPair, err := tls.LoadX509KeyPair(pubKey, privKey)
  if err != nil {
    panic(err)
  }

	//Load CA
  rootCa, err := ioutil.ReadFile(ca)
  if err != nil {
    panic(err)
  }

  //Create a new cert pool
  rootCAs := x509.NewCertPool()

  // Append our ca cert to the system pool
  if ok := rootCAs.AppendCertsFromPEM(rootCa); !ok {
    log.Println("No certs appended, using system certs only")
  }

  // Trust cert pool in our client
  clientConfig := &tls.Config{
    InsecureSkipVerify: false,
    RootCAs:            rootCAs,
    Certificates: 			[]tls.Certificate{clientKeyPair},
  }
  clientTr := &http.Transport{TLSClientConfig: clientConfig}
  secureClient := &http.Client{Transport: clientTr}

  // Still works with host-trusted CAs!
  req, err := http.NewRequest(http.MethodPost, url, body)
  if err != nil {
    panic(err)
  }
  resp, err := secureClient.Do(req)
  if err != nil {
    panic(err)
  }
  reader := bufio.NewReader(resp.Body)
  respBody, _ = ioutil.ReadAll(reader)
  resp.Body.Close()

  return respBody, nil
}

func generateSha256(file string) string{
	f, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}

	return base64.URLEncoding.EncodeToString(h.Sum(nil))
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

	if(net.ParseIP(server) == nil){
		log.Fatal("Invalid IP address!")
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
	  ServerPort: "47433",
	  AgentPort: "49433",
	  AgentJoinKey: joinKey,
	  CaCert: defaultSSLPath + name + "-ca.crt",
	  CaKey: defaultSSLPath + name + "-ca.key",
	  ServerCert: defaultSSLPath + name + "-server.crt",
	  ServerKey: defaultSSLPath + name + "-server.key",
	}
	writeFile(defaultConfigPath + name + "-config.json", config)

	//Generate hash
	caHash := generateSha256(defaultSSLPath + name + "-ca.crt")

	fmt.Printf("\n\n\n")
	fmt.Println("Once the server has been set up, add workers by running this command:")
	fmt.Printf("mozart-agent --server=%s --agent=INSERT_AGENT_IP --key=%s --ca-hash=%s", server, joinKey, caHash)
	//fmt.Printf("mozart-agent --server=", server, "agent=INSERT_AGENT_IP --key=", joinKey, "--ca-hash=", caHash)
	fmt.Printf("\n\n\n")
}

func clusterPrint(c *cli.Context) {
	//Generate hash
	caHash := generateSha256(config.CaCert)

	fmt.Printf("\n\n\n")
	fmt.Println("Once the server has been set up, add workers by running this command:")
	fmt.Printf("mozart-agent --server=%s --agent=INSERT_AGENT_IP --key=%s --ca-hash=%s", config.ServerIp, config.AgentJoinKey, caHash)
	fmt.Printf("\n\n\n")
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

func containerRun(c *cli.Context) {
	//configPath := c.String("config")
	configPath := c.Args().First()
	if(configPath == ""){
		configPath = "config.json"
	}

	f, err := os.Open(configPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	configReader := bufio.NewReader(f)

	url := "https://" + config.ServerIp + ":" + config.ServerPort + "/containers/create"
	resp, err := callSecuredServer(defaultSSLPath + config.Name + "-client.crt", defaultSSLPath + config.Name + "-client.key", defaultSSLPath + config.Name + "-ca.crt", "POST", url, configReader)
  if err != nil {
		panic(err)
	}

	respBody := Resp{}
  err = json.Unmarshal(resp, &respBody)
	if err != nil {
		panic(err)
	}

	if(!respBody.Success){
		panic(respBody.Error)
	}
}

func containerStop(c *cli.Context) {
	if(c.Args().First() == ""){
		panic("Must provide the name or id of the container.")
	}

	url := "https://" + config.ServerIp + ":" + config.ServerPort + "/containers/stop/" + c.Args().First()
	resp, err := callSecuredServer(defaultSSLPath + config.Name + "-client.crt", defaultSSLPath + config.Name + "-client.key", defaultSSLPath + config.Name + "-ca.crt", "GET", url, nil)
  if err != nil {
		panic(err)
	}

	respBody := Resp{}
  err = json.Unmarshal(resp, &respBody)
	if err != nil {
		panic(err)
	}

	if(!respBody.Success){
		panic(respBody.Error)
	}
}

func containerList(c *cli.Context) {
	url := "https://" + config.ServerIp + ":" + config.ServerPort + "/containers/list"
	resp, err := callSecuredServer(defaultSSLPath + config.Name + "-client.crt", defaultSSLPath + config.Name + "-client.key", defaultSSLPath + config.Name + "-ca.crt", "GET", url, nil)
	if err != nil {
		panic(err)
	}

	respBody := ContainerListResp{}
	err = json.Unmarshal(resp, &respBody)
	if err != nil {
		panic(err)
	}

	if(!respBody.Success){
		panic(respBody.Error)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "State", "Desired State", "Worker"})
	for _, c := range respBody.Containers {
	   table.Append([]string{c.Name, c.State, c.DesiredState, c.Worker})
	}
	table.Render() // Send output
}

func nodesList(c *cli.Context) {
	url := "https://" + config.ServerIp + ":" + config.ServerPort + "/nodes/list"
	resp, err := callSecuredServer(defaultSSLPath + config.Name + "-client.crt", defaultSSLPath + config.Name + "-client.key", defaultSSLPath + config.Name + "-ca.crt", "GET", url, nil)
	if err != nil {
		panic(err)
	}

	respBody := WorkerListResp{}
	err = json.Unmarshal(resp, &respBody)
	if err != nil {
		panic(err)
	}

	if(!respBody.Success){
		panic(respBody.Error)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"IP", "Port", "Status"})
	for _, n := range respBody.Workers {
	   table.Append([]string{n.AgentIp, n.AgentPort, n.Status})
	}
	table.Render() // Send output
}

var defaultSSLPath = "/etc/mozart/ssl/"
var defaultConfigPath = "/etc/mozart/"
var config = Config{}

func main() {
	readConfigFile("/etc/mozart/config.json")

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
					Name:  "print",
					Usage: "Print the install instructions.",
					Action: clusterPrint,
				},
				{
					Name:  "ls",
					Usage: "List all clusters this client can connect to.",
					Action: clusterList,
				},
			},
		},
		{
			Name:        "nodes",
			Usage:       "Helper commands for nodes.",
			Subcommands: []cli.Command{
				{
					Name:  "ls",
					Usage: "List all nodes in a cluster.",
					Action: nodesList,
				},
			},
		},
		{
			Name:  "run",
			Usage: "Schedules a container to be created and started.",
			Action: containerRun,
		},
		{
			Name:  "stop",
			Usage: "Schedules a container to be stopped.",
			Action: containerStop,
		},
		{
			Name:  "ls",
			Usage: "List all containers in a cluster.",
			Action: containerList,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
