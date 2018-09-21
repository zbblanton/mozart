package main

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	mathrand "math/rand"
	"strings"
)

//Container - Container information
type Container struct {
	Name         string
	State        string
	DesiredState string
	Config       ContainerConfig
	Worker       string
}

//Containers - Map to hold containers
type Containers struct {
	Containers map[string]Container
	mux        sync.Mutex
}

//ExposedPort - Struct to expose a container port
type ExposedPort struct {
	ContainerPort string
	HostPort      string
	HostIP        string
}

//Mount - Struct to mount a data to a container
type Mount struct {
	Target   string
	Source   string
	Type     string
	ReadOnly bool
}

//ContainerConfig - Config for a container
type ContainerConfig struct {
	Name         string
	Image        string
	ExposedPorts []ExposedPort
	Mounts       []Mount
	Env          []string
	AutoRemove   bool
	Privileged   bool
}

//DockerContainerHostConfigPortBindings - Used to help parse the docker API
type DockerContainerHostConfigPortBindings struct {
	HostIP   string
	HostPort string
}

//DockerContainerHostConfig - Used to help parse the docker API
type DockerContainerHostConfig struct {
	PortBindings map[string][]DockerContainerHostConfigPortBindings
	Mounts       []Mount
	AutoRemove   bool
	Privileged   bool
}

//DockerContainerConfig - Used to help parse the docker API
type DockerContainerConfig struct {
	Image        string
	Env          []string
	Labels       map[string]string
	ExposedPorts map[string]struct{}
	HostConfig   DockerContainerHostConfig
}

//CreateReq - Request for creating a container
type CreateReq struct {
	Key       string
	Container Container
}

//StopReq - Request for stopping a container
type StopReq struct {
	Key           string
	ContainerName string
}

//CreateResp - Response for creating a container
type CreateResp struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

//Req - Request
type Req struct {
	Key     string `json:"key"`
	Command string `json:"command"`
}

//Resp - Generic response
type Resp struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

//Config - Agent config
type Config struct {
	ServerKey string
	mux       sync.Mutex
}

//ControllerMsg - Controller message
type ControllerMsg struct {
	Action  string
	Data    interface{}
	Retries uint
}

func (c *Config) getServerKey() string {
	c.mux.Lock()
	serverKey := c.ServerKey
	c.mux.Unlock()
	return serverKey
}

func getContainerRuntime() string {
	return "docker"
}

func selectServer() string {
	if len(servers) == 1 {
		return servers[0]
	}
	randomNum := mathrand.Intn(len(servers))
	fmt.Println("Selected:", servers[randomNum])
	return servers[randomNum]
}

func callInsecuredServer(method string, url string, body io.Reader) (respBody []byte, err error) {
	c := &tls.Config{
		InsecureSkipVerify: true,
	}
	tr := &http.Transport{TLSClientConfig: c}
	client := &http.Client{Transport: tr}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Connection", "close") //To inform the server to close connections when completed.
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(resp.Body)
	respBody, _ = ioutil.ReadAll(reader)
	resp.Body.Close()

	return respBody, nil
}

func callSecuredServer(pubKey, privKey, ca []byte, method string, url string, body io.Reader) (respBody []byte, err error) {
	//Load our key pair
	// fmt.Println("pub::::", string(pubKey))
	// fmt.Println("priv::::", string(privKey))
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
	req.Header.Set("Connection", "close") //To inform the server to close connections when completed.
	resp, err := secureClient.Do(req)
	if err != nil {
		panic(err)
	}
	reader := bufio.NewReader(resp.Body)
	respBody, _ = ioutil.ReadAll(reader)
	resp.Body.Close()

	return respBody, nil
}

func generateCSR(privateKey *rsa.PrivateKey, IP string) (csr []byte, err error) {
	//CSR config
	csrSubject := pkix.Name{
		Organization: []string{"Mozart"}}
	csrConfig := &x509.CertificateRequest{
		Subject:     csrSubject,
		PublicKey:   privateKey,
		IPAddresses: []net.IP{net.ParseIP(IP)}}

	csr, err = x509.CreateCertificateRequest(rand.Reader, csrConfig, privateKey)
	if err != nil {
		return nil, err
	}
	return csr, err
}

func stopAllMozartContainers() {
	list, err := DockerListByID()
	if err != nil {
		panic("Could not get list of mozart containers on host")
	}
	fmt.Println("Stopping mozart containers that should not be running.")
	for _, containerID := range list {
		//fmt.Println("Stopping container:", containerID)
		DockerStopContainer(containerID)
	}
}

var config = Config{ServerKey: ""}
var agentTLSKey = []byte{}
var agentTLSCert = []byte{}
var caTLSCert = []byte{}

var containerQueue = make(chan ControllerMsg, 3)
var containerRetryQueue = make(chan ControllerMsg, 3)
var containers = Containers{
	Containers: make(map[string]Container)}

var servers []string

var agentPtr = flag.String("agent", "", "Hostname or IP to use for this agent. (Required)")
var serversPtr = flag.String("servers", "", "Hostname or IP of the mozart servers. (Required)")
var keyPtr = flag.String("key", "", "Mozart join key to join the cluster. (Required)")
var caHashPtr = flag.String("ca-hash", "", "Mozart CA hash to verify server CA. (Required)")

func main() {
	// agentPtr := flag.String("agent", "", "Hostname or IP to use for this agent. (Required)")
	// serverPtr := flag.String("server", "", "Hostname or IP of the mozart server. (Required)")
	// keyPtr := flag.String("key", "", "Mozart join key to join the cluster. (Required)")
	// caHashPtr := flag.String("ca-hash", "", "Mozart CA hash to verify server CA. (Required)")

	flag.Parse()
	if *agentPtr == "" {
		if env := os.Getenv("MOZART_AGENT_IP"); env == "" {
			conn, err := net.Dial("udp", "8.8.8.8:80")
			if err != nil {
				log.Fatal(err)
			}
			localAddr := conn.LocalAddr().(*net.UDPAddr)
			*agentPtr = localAddr.IP.String()
			fmt.Println("No agent IP provided. Automatically selecting:", *agentPtr)
			conn.Close()
			//log.Fatal("Must provide this nodes Hostname or IP.")
		} else {
			agentPtr = &env
		}
	}
	if *serversPtr == "" {
		if env := os.Getenv("MOZART_SERVERS_IP"); env == "" {
			log.Fatal("Must provide atleast one server.")
		} else {
			serversPtr = &env
		}
	}
	cleanString := strings.Replace(*serversPtr, ",", " ", -1)
 	convertToArray := strings.Fields(cleanString)
	servers = convertToArray
	if *keyPtr == "" {
		if env := os.Getenv("MOZART_JOIN_KEY"); env == "" {
			log.Fatal("Must provide a join key to join the cluster.")
		} else {
			keyPtr = &env
		}
	}
	if *caHashPtr == "" {
		if env := os.Getenv("MOZART_CA_HASH"); env == "" {
			log.Fatal("Must provide a CA hash to verify the cluster CA.")
		} else {
			caHashPtr = &env
		}
	}

	fmt.Println("Joining agent to cluster...")
	config.ServerKey = joinAgent(selectServer(), *agentPtr, *keyPtr, *caHashPtr)
	if config.ServerKey == "" {
		log.Fatal("Something went wrong when trying to join the agent.")
	}
	fmt.Println("Agent successfully joined the cluster.")

	go containerControllerQueue(containerQueue)
	go containerControllerRetryQueue(containerRetryQueue)
	go MonitorContainers(*agentPtr)
	startAgentAPI("49433")
}
