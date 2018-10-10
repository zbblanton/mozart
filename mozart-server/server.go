package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/user"
	"sync"
	"time"
	"strconv"
	"path/filepath"
	"strings"
)

//Container holds data for one container
type Container struct {
	Name         string
	State        string
	DesiredState string
	Config       ContainerConfig
	Worker       string
}

//Worker holds data for one worker
type Worker struct {
	AgentIP    string
	AgentPort  string
	ServerKey  string
	AgentKey   string
	Containers map[string]string
	Status     string
}

//Account holds data for one account
type Account struct {
	Type        string
	Name        string
	Password    string
	AccessKey   string
	SecretKey   string
	Description string
}

//Workers holds a map of workers with mux protection
type Workers struct {
	Workers map[string]Worker
	mux     sync.Mutex
}

//Containers holds a map of containers with mux protection
type Containers struct {
	Containers map[string]Container
	mux        sync.Mutex
}

//Config - Control config
type UserConfigs struct {
	Selected string
	Configs  map[string]Config
}

type Config struct {
	Server     	 string
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
	Servers			 []string
	AgentPort    string
	AgentJoinKey string
	CaCert       string
	CaKey        string
	ServerCert   string
	ServerKey    string
	mux          sync.Mutex
}

// //Config data for server
// type Config struct {
// 	Name              string
// 	ServerIP          string
// 	ServerPort        string
// 	AgentPort         string
// 	AgentJoinKey      string
// 	CaCert            string
// 	CaKey             string
// 	ServerCert        string
// 	ServerKey         string
// 	TempCurrentWorker uint
// }

//ExposedPort holds an exposed port for a container
type ExposedPort struct {
	ContainerPort string
	HostPort      string
	HostIP        string
}

//Mount holds a mount for a container
type Mount struct {
	Target   string
	Source   string
	Type     string
	ReadOnly bool
}

//ContainerConfig - Holds config for a container
type ContainerConfig struct {
	Name         string
	Image        string
	ExposedPorts []ExposedPort
	Mounts       []Mount
	Env          []string
	AutoRemove   bool
	Privileged   bool
}

//InitialJoinReq - Initial node join request
type InitialJoinReq struct {
	IP   string
	Port string
	JoinKey   string
	Csr       string
}

//NodeInitialJoinResp - Initial node join response
type NodeInitialJoinResp struct {
	CaCert     string
	ClientCert string
	Success    bool   `json:"success"`
	Error      string `json:"error"`
}

//NodeJoinReq - Node join request
type NodeJoinReq struct {
	JoinKey   string
	AgentKey  string
	Type      string
	AgentIP   string
	AgentPort string
}

//NodeJoinResp - Node join response
type NodeJoinResp struct {
	ServerKey  string
	Containers map[string]Container
	Success    bool   `json:"success"`
	Error      string `json:"error"`
}

//ContainerListResp - Container list response
type ContainerListResp struct {
	Containers map[string]Container
	Success    bool   `json:"success"`
	Error      string `json:"error"`
}

//AccountsListResp - Account list response
type AccountsListResp struct {
	Accounts map[string]Account
	Success  bool   `json:"success"`
	Error    string `json:"error"`
}

//NodeListResp - Worker list response
type NodeListResp struct {
	Workers map[string]Worker
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

//ContainerInspectResp - Container inspect response
type ContainerInspectResp struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

//RawControllerMsg - Raw Controller message
type RawControllerMsg struct {
	Action string
	Data   json.RawMessage //Delay parsing
}

//ControllerMsg - Controller message
type ControllerMsg struct {
	Action  string
	Data    interface{}
	Retries uint
}

//ControllerReconnectMsg - Controller reconnect message
type ControllerReconnectMsg struct {
	worker         Worker
	disconnectTime time.Time
}

//StateUpdateReq - Update container state
type StateUpdateReq struct {
	Key           string
	ContainerName string
	State         string
}

//Resp - Generic response
type Resp struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

//var ds = &FileDataStore{Path: "/var/lib/mozart/mozart.db"}
//var ds = &EtcdDataStore{endpoints: []string{"192.168.0.45:2379"}}
var ds DataStore
var counter = 1
var defaultConfigPath = "/etc/mozart/"
var config = ServerConfig{}
var workerQueue = make(chan ControllerMsg, 3)
var workerRetryQueue = make(chan ControllerMsg, 3)
var containerQueue = make(chan ControllerMsg, 3)
var containerRetryQueue = make(chan ControllerMsg, 3)
var serverTLSCert = []byte{}
var serverTLSKey = []byte{}
var caTLSCert = []byte{}
var defaultSSLPath = "/etc/mozart/ssl/"
var master = MasterInfo{}
var multiMaster = false

// func readConfigFile(file string) {
// 	f, err := os.Open(file)
// 	if err != nil {
// 		panic("cant open file")
// 	}
// 	defer f.Close()
//
// 	enc := json.NewDecoder(f)
// 	err = enc.Decode(&config)
// 	if err != nil {
// 		panic("cant decode")
// 	}
// }

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

func checkWorkerHealth(workerIP string, workerPort string) bool {
	//Will need to add support for the worker key!!!!!
	type Req struct {
		Key string
	}

	j := Req{Key: "NEEDTOADDSUPPORTFORTHIS!!!"}

	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(j)
	url := "https://" + workerIP + ":" + workerPort + "/health"
	resp, err := callSecuredAgent(serverTLSCert, serverTLSKey, caTLSCert, "POST", url, b)
	if err != nil {
		eventError(err)
		return false
	}

	type healthCheckResp struct {
		Health  string
		Success bool
		Error   string
	}

	respj := healthCheckResp{}
	err = json.Unmarshal(resp, &respj)
	if err != nil {
		eventError(err)
		return false
	}
	// //resp.Body.Close()
	// if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
	// 	return true
	// }

	return true
}

func createUserAccount(name string) Account{
	j := Account{
		Type: "user",
		Name: name,
		Description: "Default user account generated during install.",
	}

	//Generate Access Key
	randKey := make([]byte, 16)
	_, err := rand.Read(randKey)
	if err != nil {
		fmt.Println("Error generating a key, we are going to exit here due to possible system errors.")
		os.Exit(1)
	}
	j.AccessKey = base64.URLEncoding.EncodeToString(randKey)

	//Generate Secret Key
	randKey = make([]byte, 64)
	_, err = rand.Read(randKey)
	if err != nil {
		fmt.Println("Error generating a key, we are going to exit here due to possible system errors.")
		os.Exit(1)
	}
	j.SecretKey = base64.URLEncoding.EncodeToString(randKey)

	//Save account
	accountBytes, err := json.Marshal(j)
	if err != nil {
		eventFatal(err)
		return Account{}
	}
	ds.Put("mozart/accounts/"+"mozart", accountBytes)

	return j
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
		if caller != "" {
			home = "/home/" + caller + "/"
		}
	} else {
		home = currentUser.HomeDir + "/"
	}

	return home
}

func installServer(server string){
	name := "mozart"

	//Get the user that executed sudo's name. (Note: this may not work on every system.)
	user, _ := user.Current()
	uid := user.Uid
	gid := user.Uid
	if uid == "" || gid == "" {
		panic("Cannot get user account info.")
	}
	home := getHomeDirectory()

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
	// fmt.Println("Creating server keypair...")
	// generateSignedKeyPair(name+"-ca.crt", name+"-ca.key", name+"-server", server, defaultSSLPath)

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
		AgentPort:    "49433",
		AgentJoinKey: joinKey,
		CaCert:       defaultSSLPath + name + "-ca.crt",
		CaKey:        defaultSSLPath + name + "-ca.key",
		ServerCert:   defaultSSLPath + name + "-server.crt",
		ServerKey:    defaultSSLPath + name + "-server.key",
	}
	writeServerConfigFile(defaultConfigPath+"config.json", serverConfig)

	//Load CA
	ca, err := ioutil.ReadFile(defaultSSLPath + name + "-ca.crt")
	if err != nil {
		panic(err)
	}

	//Create a user
	userAccount := createUserAccount("mozart")

	//Create config file
	config := Config{
		Server:    server,
		AuthType:  "cred",
		Account:   userAccount.Name,
		AccessKey: userAccount.AccessKey,
		SecretKey: userAccount.SecretKey,
		Ca:        string(ca),
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
	err = os.Chown(home + ".mozart/config.json", uidInt, gidInt)
	if err != nil {
		panic(err)
	}

	err = os.Chown("/etc/mozart", uidInt, gidInt)
	if err != nil {
		panic(err)
	}

	searchDir := "/etc/mozart"
	fileList := make([]string, 0)
	err = filepath.Walk(searchDir, func(path string, f os.FileInfo, err error) error {
		fileList = append(fileList, path)
		return err
	})
	if err != nil {
		panic(err)
	}

	for _, file := range fileList {
		err = os.Chown(file, uidInt, gidInt)
		if err != nil {
			panic(err)
		}
	}

	// err = os.Chown(home + "/etc/mozart/config.json", uidInt, gidInt)
	// if err != nil {
	// 	panic(err)
	// }
}

// func passToLeader(key string, val []byte) error {
// 	url := "https://" + master.leader + ":47433" + "/passproxy"
// 	_, err := callSecuredAgent(serverTLSCert, serverTLSKey, caTLSCert, "POST", master.leader, val)
// 	if err != nil {
// 		eventError(err)
// 	}
// 	return nil
// }

func callSecuredAgent(pubKey, privKey, ca []byte, method string, url string, body io.Reader) (respBody []byte, err error) {
	//Load our key pair
	clientKeyPair, err := tls.X509KeyPair(pubKey, privKey)
	if err != nil {
		return []byte{}, err
	}

	//Create a new cert pool
	rootCAs := x509.NewCertPool()

	// Append our ca cert to the system pool
	if ok := rootCAs.AppendCertsFromPEM(ca); !ok {
		log.Println("No certs appended, using system certs only")
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
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return []byte{}, err
	}
	req.Header.Set("Connection", "close") //To inform the server to close connections when completed.
	resp, err := secureClient.Do(req)
	if err != nil {
		return []byte{}, err
	}
	reader := bufio.NewReader(resp.Body)
	respBody, _ = ioutil.ReadAll(reader)
	resp.Body.Close()

	return respBody, nil
}

func main() {
	err := os.MkdirAll("/var/lib/mozart/", 0700)
	if err != nil {
		panic(err)
	}

	//configPtr := flag.String("config", "", "Path to config file. (Default: /etc/mozart/config.json)")
	serverPtr := flag.String("server", "", "IP address for server.")
	serversPtr := flag.String("servers", "", "IP addresses for servers.")
	etcdEndpointsPtr := flag.String("etcd-endpoints", "", "IP addresses for etcd endpoints.")
	flag.Parse()

	if *etcdEndpointsPtr == "" {
		fmt.Println("Using file based datastore.")
		ds = &FileDataStore{Path: "/var/lib/mozart/mozart.db"}
	} else {
		fmt.Println("Etcd based datastore.")
		cleaned := strings.Replace(*etcdEndpointsPtr, ",", " ", -1)
	 	strSlice := strings.Fields(cleaned)
		fmt.Println("Etcd Endpoints:", strSlice)
		//ds = &EtcdDataStore{endpoints: []string{"192.168.0.45:2379"}}
		ds = &EtcdDataStore{endpoints: strSlice}
	}

	ds.Init()
	defer ds.Close()

	//Test parsing multiple IP's
	//MAY ACTUALLY ONLY NEED TO KNOW THE NUMBER OF MASTERS YOU EXPECT.
	cleaned := strings.Replace(*serversPtr, ",", " ", -1)
 // convert 'clened' comma separated string to slice
 	strSlice := strings.Fields(cleaned)
	fmt.Println("Servers:", strSlice)
	//return



	//Make sure server flag is given.
	// if *configPtr == "" {
	// 	readServerConfigFile("/etc/mozart/config.json")
	// } else {
	// 	readServerConfigFile(*configPtr)
	// }

	//Check if server config exist, if not, run the install.
	if _, err = os.Stat("/etc/mozart/config.json"); err != nil {
		fmt.Println("No config file found. Creating one...")
		if *serverPtr == "" {
			if env := os.Getenv("MOZART_SERVER_IP"); env == "" {
				log.Fatal("Must provide this server's IP.")
			} else {
				serverPtr = &env
			}
		}
		installServer(*serverPtr)
	}

	config = readServerConfigFile("/etc/mozart/config.json")
	if len(config.Servers) > 1 {
		multiMaster = true
		fmt.Println("More than one server found in config. Starting in multi-master mode.")
	}
	//Load Certs into memory
	//err := errors.New("")
	caTLSCert, err = ioutil.ReadFile(config.CaCert)
	if err != nil {
		panic(err)
	}
	//serverTLSCert, serverTLSKey = generateSignedKeyPairToMemory(config.Name+"-ca.crt", config.Name+"-ca.key", "server", config.ServerIP)
	fmt.Println("Creating server keypair...")
	generateSignedKeyPair(config.Name+"-ca.crt", config.Name+"-ca.key", "server", config.ServerIP, defaultSSLPath)
	//serverTLSCert, err = ioutil.ReadFile(config.ServerCert)
	serverTLSCert, err = ioutil.ReadFile("/etc/mozart/ssl/server.crt")
	if err != nil {
		panic(err)
	}
	//serverTLSKey, err = ioutil.ReadFile(config.ServerKey)
	serverTLSKey, err = ioutil.ReadFile("/etc/mozart/ssl/server.key")
	if err != nil {
		panic(err)
	}
	config.ServerCert = "/etc/mozart/ssl/server.crt"
	config.ServerKey = "/etc/mozart/ssl/server.key"

	master = MasterInfo{candidate: false, currentServer: config.ServerIP}
	go startRaft()

	//Start subprocesses
	go monitorWorkers()
	//go controllerContainers()
	go containerControllerQueue(containerQueue)
	go containerControllerRetryQueue(containerRetryQueue)

	go workerControllerQueue(workerQueue)
	go workerControllerRetryQueue(workerRetryQueue)

	//Start API server
	fmt.Println("Starting API server...")
	go startAPIServer(config.ServerIP, config.ServerPort, config.CaCert, config.ServerCert, config.ServerKey)

	//Start join server
	fmt.Println("Starting join server...")
	go startAccountAndJoinServer(config.ServerIP, "48433", config.CaCert, config.ServerCert, config.ServerKey)

	//Bad
	//Bad
	for { //Bad
		time.Sleep(time.Duration(15) * time.Second) //Bad
	} //Bad
	//Bad
	//Bad
}
