package main

import(
  "os"
	//"os/exec"
	//"fmt"
  //"io"
  "flag"
  "bytes"
	"log"
	//"strings"
  "fmt"
  "time"
  "sync"
  "encoding/gob"
	"encoding/json"
	"net/http"
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

type Config struct {
  MasterPort string
  WorkerPort string
  WorkerJoinKey string
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

type NodeJoinReq struct {
  JoinKey string
  AgentKey string
  Type string
  AgentIp string
  AgentPort string
}

type NodeJoinResp struct {
  ServerKey string
  Success bool `json:"success"`
  Error string `json:"error"`
}

type ContainerListResp struct {
  Containers map[string]Container
  Success bool `json:"success"`
  Error string `json:"error"`
}

type ContainerInspectResp struct {
  Success bool `json:"success"`
  Error string `json:"error"`
}

type NodeListResp struct {
  Success bool `json:"success"`
  Error string `json:"error"`
}

type Resp struct {
  Success bool `json:"success"`
  Error string `json:"error"`
}

var counter = 1
//var workers = []string{"10.0.0.33:8080", "10.0.0.67:8080"}

var config = Config{
  MasterPort: "10200",
  WorkerPort: "10201",
  WorkerJoinKey: "alkdfhghdfgdfflkjsdlkjhasdlkjhsdflkvdskjlsdakljasdfkh"}

var workers = Workers{
  Workers: make(map[string]Worker)}

var containers = Containers{
  Containers: make(map[string]Container)}

/*
func (c *Config) AddContainer(newContainer Container) {
  config.mux.Lock()
  config.Containers = append(config.Containers, newContainer)
  config.mux.Unlock()
}
*/

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

func selectWorker() Worker {
  //Simply spit out a worker for the time being
  for _, worker := range workers.Workers {
    return worker
  }

  return Worker{}
  //return Worker{NodeIp: "10.0.0.28:8080", ServerKey: "23123123132432423423dadsad"}
}

func checkWorkerHealth(workerIp string, workerPort string) bool {
  //Will need to add support for the worker key!!!!!
  type Req struct {
    Key string
  }

  j := Req{Key: "NEEDTOADDSUPPORTFORTHIS!!!"}

  b := new(bytes.Buffer)
  json.NewEncoder(b).Encode(j)
  url := "http://" + workerIp + ":" + workerPort + "/health"
  //Added the client code so that we can have a short timeout.
  var client = &http.Client{
    Timeout: time.Second * 5,
  }
  resp, err := client.Post(url, "application/json; charset=utf-8", b)
  if err != nil {
    return false
  }

  type healthCheckResp struct {
    Health string
    Success bool
    Error string
  }

  respj := healthCheckResp{}
  json.NewDecoder(resp.Body).Decode(&respj)
  resp.Body.Close()
  if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
    return true
  }

  return false
}

func ContainersCreateVerification(c ContainerConfig) bool {
  return true
}

func main() {
  serverPtr := flag.String("server", "", "IP of the server. (Required)")
  flag.Parse()
  //Make sure server flag is given.
  //ADD VERIFICATION HERE TO CHECK IF VALID IP
  if(*serverPtr == ""){
    log.Fatal("Must provide a server.")
  }

  //Load/Create config data
  if _, err := os.Stat("/home/zbblanton/mozart/mozart-server/config.data"); os.IsNotExist(err) {
    fmt.Println("Config file does not exist. Creating file...")
    writeFile("config", "config.data")
	} else {
    fmt.Println("Config file exist. Reading file...")
		readFile("config", "config.data")
  }

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

  //Start subprocesses
  go monitorWorkers()
  go controllerContainers()

  //Start API server
  startApiServer(*serverPtr, "8181")
}
