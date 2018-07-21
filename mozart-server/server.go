package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
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

//Config data for server
type Config struct {
	Name              string
	ServerIP          string
	ServerPort        string
	AgentPort         string
	AgentJoinKey      string
	CaCert            string
	CaKey             string
	ServerCert        string
	ServerKey         string
	TempCurrentWorker uint
	mux               sync.Mutex
}

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

//NodeInitialJoinReq - Initial node join request
type NodeInitialJoinReq struct {
	AgentIP   string
	AgentPort string
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

//Resp - Generic response
type Resp struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

var ds = &FileDataStore{Path: "mozart.db"}
var counter = 1
var defaultConfigPath = "/etc/mozart/"
var config = Config{}
var workerQueue = make(chan ControllerMsg, 3)
var workerRetryQueue = make(chan ControllerMsg, 3)
var containerQueue = make(chan interface{}, 3)
var containerRetryQueue = make(chan interface{}, 3)
var serverTLSCert = []byte{}
var serverTLSKey = []byte{}
var caTLSCert = []byte{}

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

func checkWorkerHealth(workerIP string, workerPort string) bool {
	//Will need to add support for the worker key!!!!!
	type Req struct {
		Key string
	}

	j := Req{Key: "NEEDTOADDSUPPORTFORTHIS!!!"}

	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(j)
	url := "https://" + workerIP + ":" + workerPort + "/health"

	//The following code will allow for TLS auth, we will need to create a function for this later.
	//-----Start-------
	//Load our key pair
	clientKeyPair, err := tls.LoadX509KeyPair(config.ServerCert, config.ServerKey)
	if err != nil {
		panic(err)
	}

	//Load CA
	rootCa, err := ioutil.ReadFile(config.CaCert)
	if err != nil {
		panic("cant open file")
	}

	//Create a new cert pool
	rootCAs := x509.NewCertPool()

	// Append our ca cert to the system pool
	if ok := rootCAs.AppendCertsFromPEM(rootCa); !ok {
		fmt.Println("No certs appended, using system certs only")
	}

	// Trust cert pool in our client
	clientConfig := &tls.Config{
		InsecureSkipVerify: false,
		RootCAs:            rootCAs,
		Certificates:       []tls.Certificate{clientKeyPair},
	}
	clientTr := &http.Transport{TLSClientConfig: clientConfig}
	secureClient := &http.Client{Transport: clientTr, Timeout: time.Second * 5}

	// Still works with host-trusted CAs!
	req, err := http.NewRequest(http.MethodPost, url, b)
	if err != nil {
		panic(err)
	}
	resp, err := secureClient.Do(req)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer resp.Body.Close()
	//-----End-------

	type healthCheckResp struct {
		Health  string
		Success bool
		Error   string
	}

	respj := healthCheckResp{}
	json.NewDecoder(resp.Body).Decode(&respj)
	//resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return true
	}

	return false
}

//Only supports 1 IP.  No multiple hostname or IP support yet.
func signCSR(caCert string, caKey string, csr []byte, ip string) (cert []byte, err error) {
	//Load CA
	catls, err := tls.LoadX509KeyPair(config.CaCert, config.CaKey)
	if err != nil {
		panic(err)
	}
	ca, err := x509.ParseCertificate(catls.Certificate[0])
	if err != nil {
		panic(err)
	}
	//Prepare certificate
	newCert := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			Organization: []string{"Mozart"},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		IPAddresses:  []net.IP{net.ParseIP(ip)},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	//Parse the CSR
	clientCSR, err := x509.ParseCertificateRequest(csr)
	if err != nil {
		panic(err)
	}

	//Sign the certificate
	certSigned, err := x509.CreateCertificate(rand.Reader, newCert, ca, clientCSR.PublicKey, catls.PrivateKey)

	//Public key
	cert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certSigned})

	return cert, nil
}

func callSecuredAgent(pubKey, privKey, ca []byte, method string, url string, body io.Reader) (respBody []byte, err error) {
	//Load our key pair
	clientKeyPair, err := tls.X509KeyPair(pubKey, privKey)
	if err != nil {
		panic(err)
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

func main() {
	ds.Init()
	defer ds.Close()

	configPtr := flag.String("config", "", "Path to config file. (Default: /etc/mozart/config.json)")
	flag.Parse()
	//Make sure server flag is given.
	if *configPtr == "" {
		readConfigFile("/etc/mozart/config.json")
	} else {
		readConfigFile(*configPtr)
	}

	//Load Certs into memory
	//err := errors.New("")
	var err error
	serverTLSCert, err = ioutil.ReadFile(config.ServerCert)
	if err != nil {
		panic(err)
	}
	serverTLSKey, err = ioutil.ReadFile(config.ServerKey)
	if err != nil {
		panic(err)
	}
	caTLSCert, err = ioutil.ReadFile(config.CaCert)
	if err != nil {
		panic(err)
	}

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
