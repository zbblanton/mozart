package main

import(
  "os"
	//"os/exec"
	//"fmt"
  "io"
  "flag"
  "bytes"
	"log"
	//"strings"
  "bufio"
  "fmt"
  "time"
  "sync"
  //"encoding/gob"
	"encoding/json"
	"net/http"
  "crypto/rand"
  "crypto/tls"
  "crypto/x509"
  "crypto/x509/pkix"
  "math/big"
  "net"
  "encoding/pem"
  "io/ioutil"
  "errors"
	//"github.com/rs/cors"
)

type Container struct {
  Name string
  State string
  DesiredState string
  Config ContainerConfig
  Worker string
}

type Worker struct {
  AgentIp string
  AgentPort string
  ServerKey string
  AgentKey string
  Containers map[string]string
  Status string
}

type Workers struct {
  Workers map[string]Worker
  mux sync.Mutex
}

type Containers struct {
  Containers map[string]Container
  mux sync.Mutex
}

/*type Config struct {
  MasterPort string
  WorkerPort string
  WorkerJoinKey string
  TempCurrentWorker uint
  mux sync.Mutex
}*/

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
  TempCurrentWorker uint
  mux sync.Mutex
}

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

type NodeInitialJoinReq struct {
  AgentIp string
  AgentPort string
  JoinKey string
  Csr string
}

type NodeInitialJoinResp struct {
  CaCert string
  ClientCert string
  Success bool `json:"success"`
  Error string `json:"error"`
}

type NodeJoinReq struct {
  JoinKey string
  AgentKey string
  Type string
  AgentIp string
  AgentPort string
}

type NodeJoinResp struct {
  ServerKey string
  Containers map[string]Container
  Success bool `json:"success"`
  Error string `json:"error"`
}

type ContainerListResp struct {
  Containers map[string]Container
  Success bool `json:"success"`
  Error string `json:"error"`
}

type NodeListResp struct {
  Workers map[string]Worker
  Success bool `json:"success"`
  Error string `json:"error"`
}

type ContainerInspectResp struct {
  Success bool `json:"success"`
  Error string `json:"error"`
}

type ControllerMsg struct {
  Action string
  Data interface{}
  Retries uint
}

type ControllerReconnectMsg struct {
  worker Worker
  disconnectTime time.Time
}

/*
type NodeListResp struct {
  Success bool `json:"success"`
  Error string `json:"error"`
}
*/
type Resp struct {
  Success bool `json:"success"`
  Error string `json:"error"`
}

var ds = &FileDataStore{Path: "mozart.db"}

var counter = 1
//var workers = []string{"10.0.0.33:8080", "10.0.0.67:8080"}

/*var config = Config{
  MasterPort: "10200",
  WorkerPort: "10201",
  WorkerJoinKey: "alkdfhghdfgdfflkjsdlkjhasdlkjhsdflkvdskjlsdakljasdfkh"}*/

var defaultConfigPath = "/etc/mozart/"
var config = Config{}
/*var config = Config{
  Name: "testcluster1",
  ServerIp: "10.0.0.28",
  ServerPort: "8181",
  AgentPort: "8080",
  AgentJoinKey: "jqzWM6_fztyapwkGF7rSg5giqwag3fFuBBauyEEo9HL62pTwDegRyojhFpWsriwip6yF2-RTihTREz2mlwt5bet8IA4SB8jN3427GvOCWWPXqsBmrOyvOhzpwV5j-joPZEHgI8e9VfQXwHKwVcchPEv5dWgRHB39olVsdur65yA=",
  CaCert: "/etc/mozart/ssl/testcluster1-ca.crt",
  CaKey: "/etc/mozart/ssl/testcluster1-ca.key",
  ServerCert: "/etc/mozart/ssl/testcluster1-server.crt",
  ServerKey: "/etc/mozart/ssl/testcluster1-server.key"}
*/

var workerQueue = make(chan ControllerMsg, 3)
var workerRetryQueue = make(chan ControllerMsg, 3)
// var workers = Workers{
//   Workers: make(map[string]Worker)}

var containerQueue = make(chan interface{}, 3)
var containerRetryQueue = make(chan interface{}, 3)
// var containers = Containers{
//   Containers: make(map[string]Container)}

var serverTlsCert = []byte{}
var serverTlsKey = []byte{}
var caTlsCert = []byte{}

/*
func (c *Config) AddContainer(newContainer Container) {
  config.mux.Lock()
  config.Containers = append(config.Containers, newContainer)
  config.mux.Unlock()
}
*/

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
/*
//taken from a google help pack
//https://groups.google.com/forum/#!topic/golang-nuts/rmKTsGHPjlA
func writeFile(dataClass string, file string){
  f, err := os.Create(file)
  if err != nil {
    panic("cant open file")
  }
  defer f.Close()

  enc := gob.NewEncoder(f)

  switch dataClass {
    case "config":
      err = enc.Encode(config)
    case "workers":
      err = enc.Encode(workers)
    case "containers":
      err = enc.Encode(containers)
    default:
      panic("Invalid file data class.")
  }

  if err != nil {
    panic("cant encode")
  }
}

func readFile(dataClass string, file string) {
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

    enc := gob.NewDecoder(f)

    switch dataClass {
      case "config":
        err = enc.Decode(&config)
      case "workers":
        err = enc.Decode(&workers)
      case "containers":
        err = enc.Decode(&containers)
      default:
        panic("Invalid file data class.")
    }

    if err != nil {
      panic("cant decode")
    }
  }
}
*/
func checkWorkerHealth(workerIp string, workerPort string) bool {
  //Will need to add support for the worker key!!!!!
  type Req struct {
    Key string
  }

  j := Req{Key: "NEEDTOADDSUPPORTFORTHIS!!!"}

  b := new(bytes.Buffer)
  json.NewEncoder(b).Encode(j)
  url := "https://" + workerIp + ":" + workerPort + "/health"





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
		Certificates: 			[]tls.Certificate{clientKeyPair},
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


/*
  //Added the client code so that we can have a short timeout.
  var client = &http.Client{
    Timeout: time.Second * 5,
  }
  resp, err := client.Post(url, "application/json; charset=utf-8", b)
  if err != nil {
    return false
  }
*/
  type healthCheckResp struct {
    Health string
    Success bool
    Error string
  }

  respj := healthCheckResp{}
  json.NewDecoder(resp.Body).Decode(&respj)
  //resp.Body.Close()
  if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
    return true
  }

  return false
}

func ContainersCreateVerification(c ContainerConfig) bool {
  return true
}

//Only supports 1 IP.  No multiple hostname or IP support yet.
func signCSR(caCert string, caKey string, csr []byte, ip string) (cert []byte, err error){
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
            Organization:  []string{"Mozart"},
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

func callSecuredAgent(pubKey, privKey, ca []byte, method string, url string, body io.Reader) (respBody []byte, err error)  {
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

func main() {
  ds.Init()
  defer ds.Close()


  configPtr := flag.String("config", "", "Path to config file. (Default: /etc/mozart/config.json)")
  flag.Parse()
  //Make sure server flag is given.
  if(*configPtr == ""){
    readConfigFile("/etc/mozart/config.json")
  } else {
    readConfigFile(*configPtr)
  }

  /*
  //Load/Create config data
  if _, err := os.Stat("/home/zbblanton/mozart/mozart-server/config.data"); os.IsNotExist(err) {
    fmt.Println("Config file does not exist. Creating file...")
    writeFile("config", "config.data")
	} else {
    fmt.Println("Config file exist. Reading file...")
		readFile("config", "config.data")
  }
  */

  /*
  //Load/Create workers data
  if _, err := os.Stat("workers.data"); os.IsNotExist(err) {
    fmt.Println("Workers file does not exist. Creating file...")
    writeFile("workers", "workers.data")
	} else {
    fmt.Println("Workers file exist. Reading file...")
		readFile("workers", "workers.data")
  }

  //Load/Create containers data
  if _, err := os.Stat("containers.data"); os.IsNotExist(err) {
    fmt.Println("Containers file does not exist. Creating file...")
    writeFile("containers", "containers.data")
	} else {
    fmt.Println("Containers file exist. Reading file...")
		readFile("containers", "containers.data")
  }
  */

  //Load Certs into memory
  err := errors.New("")
  serverTlsCert, err = ioutil.ReadFile(config.ServerCert)
  if err != nil {
    panic(err)
  }
  serverTlsKey, err = ioutil.ReadFile(config.ServerKey)
  if err != nil {
    panic(err)
  }
  caTlsCert, err = ioutil.ReadFile(config.CaCert)
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
  go startApiServer(config.ServerIp, config.ServerPort, config.CaCert, config.ServerCert, config.ServerKey)

  //Start join server
  fmt.Println("Starting join server...")
  go startJoinServer(config.ServerIp, "48433", config.CaCert, config.ServerCert, config.ServerKey)

  //Bad
  //Bad
  for ;; { //Bad
    time.Sleep(time.Duration(15) * time.Second) //Bad
  } //Bad
  //Bad
  //Bad
}
