package main

import(
  "os"
	"os/exec"
  "net"
  "bytes"
  "fmt"
  "io/ioutil"
  "bufio"
  "io"
  "time"
  "sync"
  "flag"
	"log"
	"strings"
	"encoding/json"
  "crypto/rand"
	"encoding/base64"
	"net/http"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
  "crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
  "crypto/sha256"
  "encoding/pem"
)

type ExposedPort struct {
	ContainerPort string
	HostPort string
	HostIp string
}

type Mount struct {
	Target string
	Source string
	Type string
	ReadOnly bool
}

type ContainerConfig struct {
	Name string
	Image string
	ExposedPorts []ExposedPort
	Mounts []Mount
	Env []string
	AutoRemove bool
	Privileged bool
}
/*
type DockerContainerHostConfigMounts struct {
  Target string
	Source string
	Type string
	ReadOnly bool
}
*/
type DockerContainerHostConfigPortBindings struct {
  HostIp string
  HostPort string
}

type DockerContainerHostConfig struct {
  PortBindings map[string][]DockerContainerHostConfigPortBindings
  Mounts []Mount
  AutoRemove bool
  Privileged bool
}

type DockerContainerConfig struct {
  Image string
  Env []string
  Labels map[string]string
  ExposedPorts map[string]struct{}
  HostConfig DockerContainerHostConfig
}

type CreateReq struct {
  Key string
  Container ContainerConfig
}

type StopReq struct {
  Key string
  ContainerName string
}

type CreateResp struct {
  Success bool `json:"success"`
  Error string `json:"error"`
}

type Req struct {
  Key string `json:"key"`
  Command string `json:"command"`
}

type Resp struct {
  Success bool `json:"success"`
  Error string `json:"error"`
}

type Config struct {
  ServerKey string
  mux sync.Mutex
}

func (c *Config) getServerKey() string {
  c.mux.Lock()
  serverKey := c.ServerKey
  c.mux.Unlock()
  return serverKey
}

func RootHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	defer r.Body.Close()

	j := Req{}
	json.NewDecoder(r.Body).Decode(&j)

	resp := Resp{}

	if(j.Key == "asjks882jhd88dhaSD*&Sjh28sd"){
		command := strings.Fields(j.Command)
		parts := command[1:len(command)]
		//output, err := exec.Command("echo", "Executing a command in Go").CombinedOutput()
		_, err := exec.Command(command[0], parts...).CombinedOutput()
		if err != nil {
			os.Stderr.WriteString(err.Error())
		}
	  resp = Resp{true, ""}
  } else {
    resp = Resp{false, "Invalid Key"}
  }

	json.NewEncoder(w).Encode(resp)
}

func fakeDial(proto, addr string) (conn net.Conn, err error) {
  return net.Dial("unix", "/var/run/docker.sock")
}

func MonitorContainers() {
  for {
    //Get list of containers that should be running on this worker from the master
    url := "http://10.0.0.28:8181/containers/list/10.0.0.28"
    //url := "http://10.0.0.28:8181/list"
    req, err := http.Get(url)
    if err != nil {
        panic(err)
    }
    type ContainersListResp struct {
      Containers []string
      Success bool
      Error string
    }
    j := ContainersListResp{}
    json.NewDecoder(req.Body).Decode(&j)
    req.Body.Close()

    //fmt.Print(j)
    //Loop through the containers and check the status, if not running send new state to master
    for _, container := range j.Containers {
      fmt.Println(container)
      status, err := DockerContainerStatus(container)
      if err != nil{
        panic("Failed to get container status.")
      }
      if (status != "running") {
        fmt.Println(container + ": Not running, notifying master.")
        type StateUpdateReq struct {
          Key string
          ContainerName string
          State string
        }
        j := StateUpdateReq{Key: "ADDCHECKINGFORTHIS", ContainerName: container, State: status}
        b := new(bytes.Buffer)
        json.NewEncoder(b).Encode(j)
        url := "http://10.0.0.28:8181/containers/" + container + "/state/update"
        _, err := http.Post(url, "application/json; charset=utf-8", b)
        if err != nil {
            panic(err)
        }
      }
      fmt.Println(status)
    }

    fmt.Println("Waiting 5 seconds!")
    time.Sleep(time.Duration(5) * time.Second)
  }
  os.Exit(1) //In case the for loop exits, stop the whole program.
}

func getContainerRuntime() string {
  return "docker"
}

func ConvertContainerConfigToDockerContainerConfig(c ContainerConfig) DockerContainerConfig {
  d := DockerContainerConfig{}
  d.Image = c.Image
  d.Env = c.Env
  d.Labels = make(map[string]string)
  d.Labels["mozart"] = "true"
  d.ExposedPorts = make(map[string]struct{})
  for _, port := range c.ExposedPorts {
    d.ExposedPorts[port.ContainerPort + "/tcp"] = struct{}{}
  }
  d.HostConfig.PortBindings = make(map[string][]DockerContainerHostConfigPortBindings)
  for _, port := range c.ExposedPorts {
    p := DockerContainerHostConfigPortBindings{}
    p.HostIp = port.HostIp
    p.HostPort = port.HostPort
    d.HostConfig.PortBindings[port.ContainerPort + "/tcp"] = []DockerContainerHostConfigPortBindings{p}
  }
  d.HostConfig.Mounts = c.Mounts
  d.HostConfig.AutoRemove = c.AutoRemove
  d.HostConfig.Privileged = c.Privileged

  return d
}

func DockerCallRuntimeApi(method string, url string, body io.Reader) (respBody []byte, err error)  {
  tr := &http.Transport{
    Dial: fakeDial,
  }

  client := &http.Client{Transport: tr}
  b := new(bytes.Buffer)
  json.NewEncoder(b).Encode(body)
  req, err := http.NewRequest(method, url, body)
  req.Header.Set("Content-Type", "application/json")
  resp, err := client.Do(req)
  if err != nil {
      panic(err)
  }

  reader := bufio.NewReader(resp.Body)
  respBody, _ = ioutil.ReadAll(reader)

  resp.Body.Close()

  return respBody, nil
}

func DockerCreateContainer(ContainerName string, Container DockerContainerConfig) (id string, err error){
  buff := new(bytes.Buffer)
  json.NewEncoder(buff).Encode(Container)
  url := "http://d/containers/create"
  if(ContainerName != ""){
    url = url + "?name=" + ContainerName
  }

  body, _ := DockerCallRuntimeApi("POST", url, buff)

  type ContainerCreateResp struct {
    Id string
    Warnings string
    Message string
  }
  j := ContainerCreateResp{}
  b := bytes.NewReader(body)
  json.NewDecoder(b).Decode(&j)

  //ADD VERIFICATION HERE!!!!!!!!!!!!!

  return j.Id, nil
}

func DockerStartContainer(ContainerId string) error{
  url := "http://d/containers/" + ContainerId + "/start"
  body, _ := DockerCallRuntimeApi("POST", url, bytes.NewBuffer([]byte(`{	}`)))
  type ContainerStartResp struct {
    Message string
  }
  j := ContainerStartResp{}
  b := bytes.NewReader(body)
	json.NewDecoder(b).Decode(&j)

  //ADD VERIFICATION HERE!!!!!!!!!!!!!

  return nil
}

func DockerStopContainer(ContainerId string) error{
  url := "http://d/containers/" + ContainerId + "/stop"
  body, _ := DockerCallRuntimeApi("POST", url, bytes.NewBuffer([]byte(`{	}`)))
  type ContainerStopResp struct {
    Message string
  }
  j := ContainerStopResp{}
  b := bytes.NewReader(body)
	json.NewDecoder(b).Decode(&j)

  //ADD VERIFICATION HERE!!!!!!!!!!!!!

  return nil
}

func DockerContainerStatus(ContainerName string) (status string, err error) {
  url := "http://d/containers/" + ContainerName + "/json"
  body, _ := DockerCallRuntimeApi("GET", url, nil)
  type DockerStatusResp struct {
    State struct {
      Status string
    }
  }
  j := DockerStatusResp{}
  b := bytes.NewReader(body)
	json.NewDecoder(b).Decode(&j)
  //ADD VERIFICATION HERE!!!!!!!!!!!!!

  return j.State.Status, nil
}

func CreateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	defer r.Body.Close()

  //Read json in and decode it
	j := CreateReq{}
	json.NewDecoder(r.Body).Decode(&j)

  if(getContainerRuntime() == "docker") {
    container := ConvertContainerConfigToDockerContainerConfig(j.Container)
    fmt.Print(container)
    fmt.Println(" ")
    id, _ := DockerCreateContainer(j.Container.Name, container)
    fmt.Print(id)
    fmt.Println(" ")
    DockerStartContainer(id)
  }

  p := Resp{true, ""}
  json.NewEncoder(w).Encode(p)
}

func StopHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
  r.Body.Close()

  vars := mux.Vars(r)
  containerName := vars["container"]

  if(containerName != ""){
    DockerStopContainer(containerName)
    p := Resp{true, ""}
    json.NewEncoder(w).Encode(p)
  } else {
    p := Resp{false, "Must provide a container name or ID."}
    json.NewEncoder(w).Encode(p)
  }
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
  r.Body.Close()

  type healthCheck struct {
    Health string
    Success bool
    Error string
  }

  p := healthCheck{"ok", true, ""}
  json.NewEncoder(w).Encode(p)
}

func JoinHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
  defer r.Body.Close()

  type healthCheck struct {
    Health string
    Success bool
    Error string
  }

  p := healthCheck{"ok", true, ""}
  json.NewEncoder(w).Encode(p)
}

func joinAgent(serverIp string, agentIp string, joinKey string, agentCaHash string) string{
  //Step 1: Generate a Key and CSR
  //Step 2: Send join key and CSR and receive CA
  //Step 3: Verify CA hash matches our hash and save Cert
  //Step 4: Send IP, name, join key, agent key. Receive server key

  //Step 1
  fmt.Println("Starting Join Process...")
  fmt.Println("Generating Private Key...")
  priv, _ := rsa.GenerateKey(rand.Reader, 2048)
  agentTlsKey = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}) //Save the private key
  csrSubject := pkix.Name{
      Organization:  []string{"Mozart"}}
  csrConfig := &x509.CertificateRequest{
    Subject: csrSubject,
    PublicKey: priv,
    IPAddresses:  []net.IP{net.ParseIP(agentIp)}}
  fmt.Println("Generating CSR...")
  csr, err := x509.CreateCertificateRequest(rand.Reader, csrConfig, priv)
  if err != nil {
		panic(err)
	}

  //Step 2
  fmt.Println("Encoding CSR...")
  csrString := base64.URLEncoding.EncodeToString(csr)

	c := &tls.Config{
		InsecureSkipVerify: true,
	}
	tr := &http.Transport{TLSClientConfig: c}
	client := &http.Client{Transport: tr}

  type NodeInitialJoinReq struct {
    AgentIp string
    JoinKey string
    Csr string
  }

  j := NodeInitialJoinReq{AgentIp: agentIp, JoinKey: joinKey, Csr: csrString}
  b := new(bytes.Buffer)
  json.NewEncoder(b).Encode(j)
  fmt.Println("Sending initial join request...")

	req, err := http.NewRequest(http.MethodPost, "https://10.0.0.28:8282/", b)
	if err != nil {
		panic(err)
	}
  resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}

  type NodeInitialJoinResp struct {
    CaCert string
    ClientCert string
    Success bool `json:"success"`
    Error string `json:"error"`
  }

  respj := NodeInitialJoinResp{}
  //ADD VERIFICATION FOR ERRORS
  json.NewDecoder(resp.Body).Decode(&respj)

  fmt.Println("Received response: ", respj)

  //Step 3
  if !respj.Success {
    panic(respj.Error)
  }

  //Decode CA
  fmt.Println("Decoding server CA...")
  ca, err := base64.URLEncoding.DecodeString(respj.CaCert)
  if err != nil {
    panic(err)
  }

  //Save the CA
  fmt.Println("Saving CA...")
  caTLSCert = ca

  //Decode agent CA hash, Compute hash, and compare
  agentCaHashDecoded, err := base64.URLEncoding.DecodeString(agentCaHash)
  if err != nil {
    panic(err)
  }
  fmt.Println("Comparing CA hash to our hash to validate server...")

  caHash := sha256.Sum256(ca)
  sliceCaHash := caHash[:] //Fix to convert [32]byte to []byte so that we can compare
  if(bytes.Equal(sliceCaHash, agentCaHashDecoded)){
    panic("Hashes are not equal! Cannot trust server!")
  }

  //Decode and save cert
  cert, err := base64.URLEncoding.DecodeString(respj.ClientCert)
  if err != nil {
    panic(err)
  }
  fmt.Println("Saving agent cert")
  agentTlsCert = cert



  //Step 4 (NEED TO ADD CA TO POST!!!)
  fmt.Println("The join key is: ", joinKey)
  //Generating key taken from http://blog.questionable.services/article/generating-secure-random-numbers-crypto-rand/
  //Generate random key
  randKey := make([]byte, 128)
  _, err = rand.Read(randKey)
  if err != nil {
    fmt.Println("Error generating a new worker key, we are going to exit here due to possible system errors.")
    os.Exit(1)
  }
  agentAuthKey := base64.URLEncoding.EncodeToString(randKey)

  fmt.Println("The agent auth key is: ", agentAuthKey)

  type NodeJoinReq struct {
    JoinKey string
    AgentKey string
    Type string
    AgentIp string
    AgentPort string
  }

  joinReq := NodeJoinReq{JoinKey: joinKey, AgentKey: agentAuthKey, Type: "worker", AgentIp: agentIp, AgentPort: "8080"}
  b2 := new(bytes.Buffer)
  json.NewEncoder(b2).Encode(joinReq)

  url := "https://" + serverIp + ":8181/nodes/join"

  fmt.Println("Sending secured join request...")
  //The following code will allow for TLS auth, we will need to create a function for this later.
  //-----Start-------
  //Load our key pair
  clientKeyPair, err := tls.X509KeyPair(agentTlsCert, agentTlsKey)
	if err != nil {
		panic(err)
	}

  //Create a new cert pool
	rootCAs := x509.NewCertPool()

	// Append our ca cert to the system pool
	if ok := rootCAs.AppendCertsFromPEM(caTLSCert); !ok {
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
	req, err = http.NewRequest(http.MethodPost, url, b2)
	if err != nil {
		panic(err)
	}
	secureResp, err := secureClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer secureResp.Body.Close()
  //-----End-------

  /*resp, err = http.Post(url, "application/json; charset=utf-8", b2)
  if err != nil {
      panic(err)
  }*/

  type NodeJoinResp struct {
    ServerKey string
    Success bool `json:"success"`
    Error string `json:"error"`
  }
  joinResp := NodeJoinResp{}
  //ADD VERIFICATION FOR ERRORS
  json.NewDecoder(secureResp.Body).Decode(&joinResp)
  fmt.Println("The secured join request response: ", joinResp)
  //resp.Body.Close()

  return joinResp.ServerKey
}

func startAgentApi(){
  router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/", RootHandler)
  router.HandleFunc("/create", CreateHandler)
  router.HandleFunc("/list", RootHandler)
  router.HandleFunc("/stop/{container}", StopHandler)
  router.HandleFunc("/status/{container}", RootHandler)
  router.HandleFunc("/inspect/{container}", RootHandler)
  router.HandleFunc("/health", HealthHandler)

  handler := cors.Default().Handler(router)
	//err := http.ListenAndServe(":8080", handler)


  //Setup TLS config
  rootCaPool := x509.NewCertPool()
  if ok := rootCaPool.AppendCertsFromPEM([]byte(caTLSCert)); !ok {
    panic("Cannot parse root CA.")
  }
  //load signed keypair
  signedKeyPair, err := tls.X509KeyPair(agentTlsCert, agentTlsKey)
  if err != nil {
		panic(err)
	}
  tlsCfg := &tls.Config{
      Certificates: []tls.Certificate{signedKeyPair},
      RootCAs: rootCaPool,
      ClientCAs: rootCaPool,
      ClientAuth: tls.RequireAndVerifyClientCert}

  //Setup server config
  server := &http.Server{
        Addr: "10.0.0.28" + ":" + "8080",
        Handler: handler,
        TLSConfig: tlsCfg}


  //Start API server
  err = server.ListenAndServeTLS("", "")
  log.Fatal(err)
}

var config = Config{ServerKey: ""}
var agentTlsKey = []byte{}
var agentTlsCert = []byte{}
var caTLSCert = []byte{}

func main() {
  agentPtr := flag.String("agent", "", "Hostname or IP to use for this agent. (Required)")
  serverPtr := flag.String("server", "", "Hostname or IP of the mozart server. (Required)")
  keyPtr := flag.String("key", "", "Mozart join key to join the cluster. (Required)")
  caHashPtr := flag.String("ca-hash", "", "Mozart CA hash to verify server CA. (Required)")
  flag.Parse()
  if(*agentPtr == ""){
    log.Fatal("Must provide this nodes Hostname or IP.")
  }
  if(*serverPtr == ""){
    log.Fatal("Must provide a server.")
  }
  if(*keyPtr == ""){
    log.Fatal("Must provide a join key to join the cluster.")
  }
  if(*caHashPtr == ""){
    log.Fatal("Must provide a CA hash to verify the cluster CA.")
  }
  fmt.Println("Joining agent to " + *serverPtr + "...")
  config.ServerKey = joinAgent(*serverPtr, *agentPtr, *keyPtr, *caHashPtr)
  if(config.ServerKey == ""){
    log.Fatal("Something went wrong when trying to join the agent.")
  }
  fmt.Println("Agent successfully joined the cluster.")
  //go MonitorContainers()
	startAgentApi()
}
