package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/olekukonko/tablewriter"
	"gopkg.in/urfave/cli.v1"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"errors"
	"strconv"
	"time"
	"strings"
	mathrand "math/rand"
)

//Config - Control config
type UserConfigs struct {
	Selected string
	Configs  map[string]Config
}

type Config struct {
	Servers      []string
	AuthType   	 string
	Account      string
	AccessKey    string
	SecretKey		 string
	ClientKey    string
	ClientCert   string
	Ca   				 string
}

//ServerConfig - Server config
type ServerConfig struct {
	Name         string
	ServerIP     string
	ServerPort   string
	Servers      []string
	AgentPort    string
	AgentJoinKey string
	CaCert       string
	CaKey        string
	//ServerCert   string
	//ServerKey    string
}


//Container - Container struct
type Container struct {
	Name         string
	State        string
	DesiredState string
	Worker       string
}

//ContainerListResp - Response for container list
type ContainerListResp struct {
	Containers map[string]Container
	Success    bool
	Error      string
}

//ClusterConfigResp - Response for cluster config
type ClusterConfigResp struct {
	Servers []string
	Ca      string
	CaHash  string
	JoinKey string
	Success bool
	Error   string
}

//Worker - Worker struct
type Worker struct {
	AgentIP   string
	AgentPort string
	Status    string
}

//WorkerListResp - Response for the worker list
type WorkerListResp struct {
	Workers map[string]Worker
	Success bool
	Error   string
}

//Account - Account struct
type Account struct {
	Type        string
	Name        string
	Password    string
	AccessKey   string
	SecretKey   string
	Description string
}

//AccountsListResp - Response for accounts list
type AccountsListResp struct {
	Accounts map[string]Account
	Success  bool   `json:"success"`
	Error    string `json:"error"`
}

//Resp - Generic response
type Resp struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

//taken from a google help pack
//https://groups.google.com/forum/#!topic/golang-nuts/rmKTsGHPjlA


func writeConfigFile(file, name string, newConfig Config, onlyChangeSelected bool) {
	config := UserConfigs{
		Configs: make(map[string]Config),
	}
	//configs := make(map[string]Config)
	var f *os.File
	if _, err := os.Stat(file); err == nil {
		f, err = os.OpenFile(file, os.O_RDWR, 0644)
		if err != nil {
			panic("cant open file")
		}

		//Get all the current configs inside the file
		dec := json.NewDecoder(f)
		err := dec.Decode(&config)
		if err != nil {
			panic("cant decode")
		}
	} else {
		f, err = os.Create(file)
		if err != nil {
			panic("cant open file")
		}
	}
	defer f.Close()

	config.Selected = name
	if !onlyChangeSelected {
		config.Configs[name] = newConfig
	}

	//Wipe file before writing
	f.Truncate(0)
  f.Seek(0,0)

	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
	err := enc.Encode(config)
	if err != nil {
		panic("cant encode")
	}
}

func readConfigFile(file string) Config {
	config := UserConfigs{
		Configs: make(map[string]Config),
	}
	//config := make(map[string]Config)
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

	if _, ok := config.Configs[config.Selected]; !ok {
		panic("Could not find the selected cluster in the config file.")
	}

	return config.Configs[config.Selected]
}

func writeServerConfigFile(file string, config ServerConfig) {
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

func readServerConfigFile(file string) ServerConfig {
	config := ServerConfig{}
	f, err := os.Open(file)
	if err != nil {
		panic("Cant open file. Please note you may need to run sudo to access the servers config file.")
	}
	defer f.Close()

	enc := json.NewDecoder(f)
	err = enc.Decode(&config)
	if err != nil {
		panic("cant decode")
	}

	return config
}

func getHomeDirectory() string {
	//Find the users home directory
	//If needed get the user that executed sudo's name. (Note: this may not work on every system.)
	var home string
	currentUser, err := user.Current()
  if err != nil {
      log.Fatal(err)
  }
	//fmt.Println(currentUser.Username)
	if currentUser.Username == "root" {
		caller := os.Getenv("SUDO_USER")
		if caller == "" {
			panic("Cannot retrieve user's home directory")
		}
		home = "/home/" + caller + "/"
	} else {
		home = currentUser.HomeDir + "/"
	}

	return home
}

func callServerByCred(uri string, body io.Reader) (respBody []byte, err error) {
	home := getHomeDirectory()

	config := readConfigFile(home + ".mozart/config.json")
	ca := config.Ca
	//method := "POST"

	//Create a new cert pool
	rootCAs := x509.NewCertPool()

	//Load CA
	if _, err := os.Stat(ca); err == nil {
		rootCa, err := ioutil.ReadFile(ca)
		if err != nil {
			panic(err)
		}

		// Append our ca cert to the system pool
		if ok := rootCAs.AppendCertsFromPEM(rootCa); !ok {
			log.Println("No certs appended, using system certs only")
		}
	} else {
		// Append our ca cert to the system pool
		if ok := rootCAs.AppendCertsFromPEM([]byte(ca)); !ok {
			log.Println("No certs appended, using system certs only")
		}
	}

	// Trust cert pool in our client
	clientConfig := &tls.Config{
		InsecureSkipVerify: false,
		RootCAs:            rootCAs,
	}
	clientTr := &http.Transport{TLSClientConfig: clientConfig}
	secureClient := &http.Client{Transport: clientTr}

	// Still works with host-trusted CAs!
	var selectedMaster string
	if len(config.Servers) == 1 {
		//selectedMaster = config.Servers[mathrand.Intn(len(config.Servers))]
		selectedMaster = config.Servers[0]
	} else {
		randomNum := mathrand.Intn(len(config.Servers))
		selectedMaster = config.Servers[randomNum]
	}
	fmt.Println("Calling:", selectedMaster)

	url := "https://" + selectedMaster + ":48433/" + uri
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return respBody, err
	}
	req.Header.Set("Connection", "close") //To inform the server to close connections when completed.
	req.Header.Set("Account", config.Account)
  req.Header.Set("Access-Key", config.AccessKey)
  req.Header.Set("Secret-Key", config.SecretKey)
	resp, err := secureClient.Do(req)
	if err != nil {
		return respBody, err
	}
	reader := bufio.NewReader(resp.Body)
	respBody, _ = ioutil.ReadAll(reader)
	resp.Body.Close()

	return respBody, nil
}

func callServerByKey(uri string, body io.Reader) (respBody []byte, err error) {
	home := getHomeDirectory()
	config := readConfigFile(home + ".mozart/config.json")
	pubKey := config.ClientKey
	privKey := config.ClientCert

	//Load our key pair
	clientKeyPair, err := tls.LoadX509KeyPair(pubKey, privKey)
	if err != nil {
		panic(err)
	}

	ca := config.Ca
	//method := "POST"

	//Create a new cert pool
	rootCAs := x509.NewCertPool()

	//Load CA
	if _, err := os.Stat(ca); err == nil {
		rootCa, err := ioutil.ReadFile(ca)
		if err != nil {
			panic(err)
		}

		// Append our ca cert to the system pool
		if ok := rootCAs.AppendCertsFromPEM(rootCa); !ok {
			log.Println("No certs appended, using system certs only")
		}
	} else {
		// Append our ca cert to the system pool
		if ok := rootCAs.AppendCertsFromPEM([]byte(ca)); !ok {
			log.Println("No certs appended, using system certs only")
		}
	}

	// Trust cert pool in our client
	clientConfig := &tls.Config{
		InsecureSkipVerify: false,
		RootCAs:            rootCAs,
		Certificates:       []tls.Certificate{clientKeyPair},
	}
	clientTr := &http.Transport{TLSClientConfig: clientConfig}
	secureClient := &http.Client{Transport: clientTr}

	// Still works with host-trusted CAs!
	var selectedMaster string
	if len(config.Servers) == 1 {
		//selectedMaster = config.Servers[mathrand.Intn(len(config.Servers))]
		selectedMaster = config.Servers[0]
	} else {
		randomNum := mathrand.Intn(len(config.Servers))
		selectedMaster = config.Servers[randomNum]
	}
	fmt.Println("Calling:", selectedMaster)

	url := "https://" + selectedMaster + ":47433/" + uri
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return respBody, err
	}
	req.Header.Set("Connection", "close") //To inform the server to close connections when completed.
	resp, err := secureClient.Do(req)
	if err != nil {
		return respBody, err
	}
	reader := bufio.NewReader(resp.Body)
	respBody, _ = ioutil.ReadAll(reader)
	resp.Body.Close()

	return respBody, nil
}

func callServer(url string, body io.Reader) (respBody []byte, err error) {
	//Get each flag
	//In this order of if statements
	//Check if config file exist
	//if not
	//Check if Cred file exist
	//if not
	//Check if flags exist for server, (client private key path, client public key path) or (access key and secret key), ((CA path or CA) or insecure flag).

	//depending on whats set above:
	//callServerByKey or callServerByCred

	home := getHomeDirectory()
	//fmt.Println(home)
	if _, err := os.Stat(home + ".mozart/config.json"); err == nil {
		config := readConfigFile(home + ".mozart/config.json")
		if config.AuthType == "key"{
		 	resp, err := callServerByKey(url, body)
			return resp, err
		} else {
			resp, err := callServerByCred(url, body)
			return resp, err
		}
	}

	// if _, err := os.Stat("/etc/mozart/config.json"); err == nil {
	// 	resp, err := callServerByKey(url, body)
	// 	return resp, err
	// }

	return nil, errors.New("Could not find or do not have permission to open user's mozart config.")
}

func generateSha256(file string) string {
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

func formatServers(serversList []string) string {
	//Format servers string
	var servers string
	for key, server := range serversList {
		if (len(config.Servers) - 1) == key {
			servers = server
		} else {
			servers = server + ","
		}
	}

	return servers
}

func clusterSwitch(c *cli.Context) {
	switchTo := c.Args().First()
	if switchTo == "" {
		fmt.Println("Must provide a cluster name.")
		return
	}

	var config UserConfigs
	home := getHomeDirectory()
	f, err := os.Open(home + ".mozart/config.json")
	if err != nil {
		panic("cant open file")
	}
	enc := json.NewDecoder(f)
	err = enc.Decode(&config)
	if err != nil {
		panic("cant decode")
	}
	f.Close()

	if _, ok := config.Configs[switchTo]; !ok {
		fmt.Println("Could not find cluster in the user config file.")
		return
	}
	writeConfigFile(home + ".mozart/config.json", switchTo, Config{}, true)
	fmt.Println("Switched cluster from", config.Selected, "to", switchTo)
}

func clusterCreate(c *cli.Context) {
	name := c.String("name")
	server := c.String("server")
	serversCSV := c.String("servers")
	if name == "" {
		log.Fatal("Please provide a name for the server.")
	}

	if server == "" {
		log.Fatal("Please provide the Mozart server address.")
	}

	var servers []string
	if serversCSV == "" {
			servers = []string{server}
	} else {
		cleanString := strings.Replace(serversCSV, ",", " ", -1)
	 	convertToArray := strings.Fields(cleanString)
		servers = convertToArray
	}

	if net.ParseIP(server) == nil {
		log.Fatal("Invalid IP address!")
	}

	//Get the user that executed sudo's name. (Note: this may not work on every system.)
	user := os.Getenv("SUDO_USER")
	uid := os.Getenv("SUDO_UID")
	gid := os.Getenv("SUDO_GID")
	if user == "" || uid == "" || gid == "" {
		panic("Cannot retrieve the sudo caller's user")
	}
	home := "/home/" + user + "/"

	//Prep folder structure
	err := os.MkdirAll("/etc/mozart/ssl", 0755)
	if err != nil {
		panic(err)
	}

	err = os.MkdirAll(home + ".mozart/keys", 0755)
	if err != nil {
		panic(err)
	}

	fmt.Println("Creating Mozart CA...")
	generateCaKeyPair(name + "-ca")
	//fmt.Println("Creating server keypair...")
	//generateSignedKeyPair(name+"-ca.crt", name+"-ca.key", name+"-server", server, defaultSSLPath)
	fmt.Println("Creating client keypair...")
	generateSignedKeyPair(name+"-ca.crt", name+"-ca.key", name+"-client", server, home+".mozart/keys/")

	//Generate worker join key
	randKey := make([]byte, 128)
	_, err = rand.Read(randKey)
	if err != nil {
		fmt.Println("Error generating a new worker key, we are going to exit here due to possible system errors.")
		os.Exit(1)
	}
	joinKey := base64.URLEncoding.EncodeToString(randKey)

	//Create server config file
	serverConfig := ServerConfig{
		Name:         name,
		ServerIP:     server,
		ServerPort:   "47433",
		Servers:			servers,
		AgentPort:    "49433",
		AgentJoinKey: joinKey,
		CaCert:       defaultSSLPath + name + "-ca.crt",
		CaKey:        defaultSSLPath + name + "-ca.key",
		//ServerCert:   defaultSSLPath + name + "-server.crt",
		//ServerKey:    defaultSSLPath + name + "-server.key",
	}
	writeServerConfigFile(defaultConfigPath+"config.json", serverConfig)

	//Load CA
	ca, err := ioutil.ReadFile(defaultSSLPath + name + "-ca.crt")
	if err != nil {
		panic(err)
	}

	//Create config file
	config := Config{
		Servers:     servers,
		AuthType:   "key",
		ClientKey:  home + ".mozart/keys/" + name + "-client.crt",
		ClientCert: home + ".mozart/keys/" + name + "-client.key",
		Ca:         string(ca),
	}
	writeConfigFile(home + ".mozart/config.json", name, config, false)

	//Set permissions to the sudo caller
	uidInt, err := strconv.Atoi(uid)
	if err != nil {
		panic(err)
	}
	gidInt, err := strconv.Atoi(gid)
	if err != nil {
		panic(err)
	}
	err = os.Chown(home + ".mozart", uidInt, gidInt)
	if err != nil {
		panic(err)
	}
	err = os.Chown(home + ".mozart/keys", uidInt, gidInt)
	if err != nil {
		panic(err)
	}
	err = os.Chown(home+".mozart/keys/"+name+"-client.crt", uidInt, gidInt)
	if err != nil {
		panic(err)
	}
	err = os.Chown(home+".mozart/keys/"+name+"-client.key", uidInt, gidInt)
	if err != nil {
		panic(err)
	}
	err = os.Chown(home + ".mozart/config.json", uidInt, gidInt)
	if err != nil {
		panic(err)
	}

	//Generate hash
	caHash := generateSha256(defaultSSLPath + name + "-ca.crt")

	fmt.Printf("\n\n\n")
	fmt.Println("Once the server has been set up, add workers by running this command:")
	//fmt.Printf("mozart-agent --server=%s --agent=INSERT_AGENT_IP --key=%s --ca-hash=%s", server, joinKey, caHash)
	fmt.Printf(`docker run --name mozart-agent -d --restart=always --privileged -v /var/run/docker.sock:/var/run/docker.sock -p 49433:49433 -e "MOZART_MASTERS=%s" -e "MOZART_AGENT_IP=INSERT_HOST_IP_HERE" -e "MOZART_JOIN_KEY=%s" -e "MOZART_CA_HASH=%s" zbblanton/mozart-agent`, serversCSV, joinKey, caHash)
	fmt.Printf("\n\n\n")
}

func clusterPrint(c *cli.Context) {
	home := getHomeDirectory()
	if _, err := os.Stat(home+".mozart/config.json"); err == nil {
		var config ClusterConfigResp
		uri := "cluster/config/"
		resp, err := callServer(uri, nil)
		if err != nil {
			fmt.Println("Could not retrieve from server.")
		} else {
			err = json.Unmarshal(resp, &config)
			if err != nil {
				panic(err)
			}

			servers := formatServers(config.Servers)

			fmt.Printf("\n\n\n")
			fmt.Println("Once the server has been set up, add workers by running this command:")
			fmt.Printf(`docker run --name mozart-agent -d --restart=always --privileged -v /var/run/docker.sock:/var/run/docker.sock -p 49433:49433 -e "MOZART_MASTERS=%s" -e "MOZART_AGENT_IP=INSERT_HOST_IP_HERE" -e "MOZART_JOIN_KEY=%s" -e "MOZART_CA_HASH=%s" zbblanton/mozart-agent`, servers, config.JoinKey, config.CaHash)
			fmt.Printf("\n\n\n")

			return
		}
	}

	fmt.Println("Trying to open server config locally.")
	serverConfig := readServerConfigFile("/etc/mozart/config.json")

	//Generate hash
	caHash := generateSha256(serverConfig.CaCert)

	fmt.Printf("\n\n\n")
	fmt.Println("Once the server has been set up, add workers by running this command:")
	fmt.Printf(`docker run --name mozart-agent -d --restart=always --privileged -v /var/run/docker.sock:/var/run/docker.sock -p 49433:49433 -e "MOZART_MASTERS=%s" -e "MOZART_AGENT_IP=INSERT_HOST_IP_HERE" -e "MOZART_JOIN_KEY=%s" -e "MOZART_CA_HASH=%s" zbblanton/mozart-agent`, serverConfig.ServerIP, serverConfig.AgentJoinKey, caHash)
	fmt.Printf("\n\n\n")
}

func clusterList(c *cli.Context) {
	var config UserConfigs
	home := getHomeDirectory()
	f, err := os.Open(home + ".mozart/config.json")
	if err != nil {
		panic("cant open file")
	}
	defer f.Close()

	enc := json.NewDecoder(f)
	err = enc.Decode(&config)
	if err != nil {
		panic("cant decode")
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Cluster Name", "Server", "Authentication Type", "Selected"})
	for name, c := range config.Configs {
		if(name == config.Selected){
			serversString := formatServers(c.Servers)
			table.Append([]string{name, serversString, c.AuthType, "*"})
		} else {
			serversString := formatServers(c.Servers)
			table.Append([]string{name, serversString, c.AuthType, ""})
		}
	}
	table.Render() // Send output
}

func clusterCaPrint(c *cli.Context) {
	home := getHomeDirectory()
	if _, err := os.Stat(home+".mozart/config.json"); err == nil {
		type ClusterConfigResp struct {
			Server  string
			Ca      string
			CaHash  string
			JoinKey string
			Success bool
			Error   string
		}
		var config ClusterConfigResp
		uri := "cluster/config/"
		resp, err := callServer(uri, nil)
		if err != nil {
			fmt.Println("Could not retrieve from server.")
		} else {
			err = json.Unmarshal(resp, &config)
			if err != nil {
				panic(err)
			}

			fmt.Println(config.Ca)

			return
		}
	}

	fmt.Println("Trying to open server config locally.")
	serverConfig := readServerConfigFile("/etc/mozart/config.json")
	//Load CA
	rootCa, err := ioutil.ReadFile(defaultSSLPath + serverConfig.Name + "-ca.crt")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(rootCa))
}

func clusterNewKeyPair(c *cli.Context) {
	name := c.String("name")
	server := c.String("server")
	if name == "" {
		log.Fatal("Please provide a name for the keypair.")
	}

	if server == "" {
		log.Fatal("Please provide the server address. for the keypair")
	}

	generateSignedKeyPair("mozart-ca.crt", "mozart-ca.key", name, server, "")
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

func accountsCreate(c *cli.Context) {
	accountName := c.Args().First()
	if accountName == "" {
		fmt.Println("Must provide an account name.")
		return
	}

	newAccount := Account{
		Type: "service",
		Name: accountName}

	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(newAccount)
	uri := "accounts/create"
	resp, err := callServer(uri, b)
	if err != nil {
		panic(err)
	}

	type AccountCreateResp struct {
		Account Account
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}

	respBody := AccountCreateResp{}
	err = json.Unmarshal(resp, &respBody)
	if err != nil {
		panic(err)
	}

	if !respBody.Success {
		fmt.Println(respBody.Error)
	}
	fmt.Println("Created account for", respBody.Account.Name)
	fmt.Println("Access Key:", respBody.Account.AccessKey)
	fmt.Println("Secret Key:", respBody.Account.SecretKey)
	fmt.Println("")
	fmt.Println("Please save these keys! This is the only time you will see them.")
}

func accountsList(c *cli.Context) {
	uri := "accounts/list"
	resp, err := callServer(uri, nil)
	if err != nil {
		panic(err)
	}

	respBody := AccountsListResp{}
	err = json.Unmarshal(resp, &respBody)
	if err != nil {
		panic(err)
	}

	if !respBody.Success {
		panic(respBody.Error)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Type", "Name", "Description"})
	for _, c := range respBody.Accounts {
		table.Append([]string{c.Type, c.Name, c.Description})
	}
	table.Render() // Send output
}

func containerRun(c *cli.Context) {
	//configPath := c.String("config")
	configPath := c.Args().First()
	if configPath == "" {
		configPath = "config.json"
	}

	f, err := os.Open(configPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	configReader := bufio.NewReader(f)

	uri := "containers/create"
	resp, err := callServer(uri, configReader)
	if err != nil {
		panic(err)
	}

	respBody := Resp{}
	err = json.Unmarshal(resp, &respBody)
	if err != nil {
		panic(err)
	}

	if !respBody.Success {
		panic(respBody.Error)
	}
}

func containerStop(c *cli.Context) {
	if c.Args().First() == "" {
		panic("Must provide the name or id of the container.")
	}

	uri := "containers/stop/" + c.Args().First()
	resp, err := callServer(uri, nil)
	if err != nil {
		panic(err)
	}

	respBody := Resp{}
	err = json.Unmarshal(resp, &respBody)
	if err != nil {
		panic(err)
	}

	if !respBody.Success {
		fmt.Println(respBody.Error)
	}
}

func containerList(c *cli.Context) {
	uri := "containers/list"
	resp, err := callServer(uri, nil)
	if err != nil {
		panic(err)
	}

	respBody := ContainerListResp{}
	err = json.Unmarshal(resp, &respBody)
	if err != nil {
		panic(err)
	}

	if !respBody.Success {
		panic(respBody.Error)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "State", "Desired State", "Worker"})
	for _, c := range respBody.Containers {
		table.Append([]string{c.Name, c.State, c.DesiredState, c.Worker})
	}
	table.Render() // Send output
}

func workersList(c *cli.Context) {
	uri := "workers/list"
	resp, err := callServer(uri, nil)
	if err != nil {
		panic(err)
	}

	respBody := WorkerListResp{}
	err = json.Unmarshal(resp, &respBody)
	if err != nil {
		panic(err)
	}

	if !respBody.Success {
		panic(respBody.Error)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"IP", "Port", "Status"})
	for _, n := range respBody.Workers {
		table.Append([]string{n.AgentIP, n.AgentPort, n.Status})
	}
	table.Render() // Send output
}

var defaultSSLPath = "/etc/mozart/ssl/"
var defaultConfigPath = "/etc/mozart/"
var config = Config{}

func main() {
	mathrand.Seed(time.Now().Unix())

	app := cli.NewApp()
	app.Name = "mozartctl"
	app.Usage = "CLI for Mozart clusters."
	app.Version = "0.1.0"
	app.Commands = []cli.Command{
		{
			Name:  "cluster",
			Usage: "Helper commands for clusters.",
			Subcommands: []cli.Command{
				{
					Name:   "switch",
					Usage:  "switch to another cluster",
					Action: clusterSwitch,
				},
				{
					Name:   "create",
					Usage:  "Generate a new cluster config and files.",
					Flags:  []cli.Flag{flagClusterName, flagClusterServer, flagClusterServers},
					Action: clusterCreate,
				},
				{
					Name:   "print",
					Usage:  "Print the install instructions.",
					Action: clusterPrint,
				},
				{
					Name:   "ls",
					Usage:  "List all clusters this client can connect to.",
					Action: clusterList,
				},
				{
					Name:   "ca",
					Usage:  "Print the CA cert.",
					Action: clusterCaPrint,
				},
				{
					Name:   "new-keypair",
					Usage:  "Print the CA cert.",
					Flags:  []cli.Flag{flagClusterName, flagClusterServer},
					Action: clusterNewKeyPair,
				},
			},
		},
		{
			Name:  "workers",
			Usage: "Helper commands for nodes.",
			Subcommands: []cli.Command{
				{
					Name:   "ls",
					Usage:  "List all workers in a cluster.",
					Action: workersList,
				},
			},
		},
		{
			Name:  "accounts",
			Usage: "Helper commands for accounts.",
			Subcommands: []cli.Command{
				{
					Name:   "create",
					Usage:  "Create an account in a cluster.",
					Action: accountsCreate,
				},
				{
					Name:   "ls",
					Usage:  "List all the accounts in a cluster.",
					Action: accountsList,
				},
			},
		},
		{
			Name:   "run",
			Usage:  "Schedules a container to be created and started.",
			Action: containerRun,
		},
		{
			Name:   "stop",
			Usage:  "Schedules a container to be stopped.",
			Action: containerStop,
		},
		{
			Name:   "ls",
			Usage:  "List all containers in a cluster.",
			Action: containerList,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
