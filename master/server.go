package main

import(
  "os"
	//"os/exec"
	//"fmt"
  //"io"
  "bytes"
	"log"
	//"strings"
  "fmt"
  "time"
  "sync"
  "encoding/gob"
	"encoding/json"
  "crypto/rand"
	"encoding/base64"
	"net/http"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

type Container struct {
  Name string
  State string
  DesiredState string
  Config ContainerConfig
  Worker string
}

type Worker struct {
  NodeIp string
  Key string
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
  Key string
  Type string
  HostIp string
}

type NodeJoinResp struct {
  Key string
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
  WorkerJoinKey: "alk;l;kd9wisas;lkdlkdsl;kjdsf;lkadsflkjfmdjj3239difdjdkasddf8ds9sd8389sdlhdsflkaefl98398diudslikhuads89498y20290si;df89"}

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
  //For now maybe just a round robin.
  return Worker{"10.0.0.28:8080", "23123123132432423423dadsad"}
}

//Used to schedule actions such as creating or deleting a container
func schedulerCreateContainer(c ContainerConfig) {
  worker := selectWorker()
  newContainer := Container{
    Name: c.Name,
    State: "",
    DesiredState: "running",
    Config: c,
    Worker: worker.NodeIp}

  containers.mux.Lock()
  //config.Containers = append(config.Containers, newContainer)
  containers.Containers[c.Name] = newContainer
  writeFile("containers", "containers.data")
  containers.mux.Unlock()
}

func controllerWorkers() {
  for {
    //Loop through workers and make sure the desiredState matches the state, if not, perform DesiredState action.
    workers.mux.Lock()
    for _, worker := range workers.Workers {
      fmt.Print(worker)
    }
    workers.mux.Unlock()
    fmt.Println("Waiting 30 seconds!")
    time.Sleep(time.Duration(30) * time.Second)
  }
  os.Exit(1) //In case the for loop exits, stop the whole program.
}

func controllerContainers() {
  for {
    //Loop through containers and make sure the desiredState matches the state, if not, perform DesiredState action.
    containers.mux.Lock()
    for key, container := range containers.Containers {
      if(container.State != container.DesiredState){
        if(container.DesiredState == "running"){
          //Run function to start a container
          //Below we assume that the containers actually start and put in a running state. Will need to add actual checks.
          controllerContainersStart(container)
          container.State = "running"
          containers.Containers[key] = container
          writeFile("containers", "containers.data")
          fmt.Print(container)
        }
      }
    }
    containers.mux.Unlock()
    fmt.Println("Waiting 15 seconds!")
    time.Sleep(time.Duration(15) * time.Second)
  }
  os.Exit(1) //In case the for loop exits, stop the whole program.
}

func controllerContainersStart(c Container){
  //Will need to add support for the worker key!!!!!
  type CreateReq struct {
    Key string
    Container ContainerConfig
  }

  j := CreateReq{Key: "NEEDTOADDSUPPORTFORTHIS!!!", Container: c.Config}

  b := new(bytes.Buffer)
  json.NewEncoder(b).Encode(j)
  url := "http://" + c.Worker + "/create"
  _, err := http.Post(url, "application/json; charset=utf-8", b)
  if err != nil {
      panic(err)
  }
}

/*
func RunHandler(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json; charset=UTF-8")
  w.WriteHeader(http.StatusOK)

  defer r.Body.Close()

  j := Req{}
  json.NewDecoder(r.Body).Decode(&j)

  resp := Resp{}

  if(j.Key == "d9d9as90opspod;a;lk0s09dkdka;"){
    if(counter == len(workers)){
      counter = 1
    } else {
      counter++
    }

    url := "http://" + workers[counter - 1]

    u := Req{Key: "asjks882jhd88dhaSD*&Sjh28sd", Command: j.Command}
  	b := new(bytes.Buffer)
  	json.NewEncoder(b).Encode(u)
  	res, _ := http.Post(url, "application/json; charset=utf-8", b)
  	io.Copy(os.Stdout, res.Body)

    resp = Resp{true, ""}
  } else {
    resp = Resp{false, "Invalid Key"}
  }

  json.NewEncoder(w).Encode(resp)
}
*/

func ContainersCreateVerification(c ContainerConfig) bool {
  return true
}

func ContainersCreateHandler(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json; charset=UTF-8")
  w.WriteHeader(http.StatusOK)
  defer r.Body.Close()

  j := ContainerConfig{}
  json.NewDecoder(r.Body).Decode(&j)
  if(ContainersCreateVerification(j)){
      go schedulerCreateContainer(j)
      resp := Resp{true, ""}
      json.NewEncoder(w).Encode(resp)
  }else {
      resp := Resp{false, "Invalid data"} //Add better error.
      json.NewEncoder(w).Encode(resp)
  }
}


func ContainersListHandler(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json; charset=UTF-8")
  w.WriteHeader(http.StatusOK)
  defer r.Body.Close()

  resp := ContainerListResp{containers.Containers, true, ""}
  json.NewEncoder(w).Encode(resp)
}

func NodeJoinHandler(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json; charset=UTF-8")
  w.WriteHeader(http.StatusOK)
  defer r.Body.Close()

  j := NodeJoinReq{}
  json.NewDecoder(r.Body).Decode(&j)

  //ADD VERIFICATION!!!!

  if(j.Key == config.WorkerJoinKey){
    //Generating key taken from http://blog.questionable.services/article/generating-secure-random-numbers-crypto-rand/
    //Generate random key
    r := make([]byte, 128)
    _, err := rand.Read(r)
    if err != nil {
      fmt.Println("Error generating a new worker key, we are going to exit here due to possible system errors.")
	    os.Exit(1)
	  }
    key := base64.URLEncoding.EncodeToString(r)
    //Save key to config
    newWorker := Worker{NodeIp: j.HostIp, Key: key}
    workers.mux.Lock()
    workers.Workers[j.HostIp] = newWorker
    writeFile("workers", "workers.data")
    workers.mux.Unlock()
    //Send key to worker
    resp := NodeJoinResp{key, true, ""}
    json.NewEncoder(w).Encode(resp)
  } else {
    resp := NodeJoinResp{"", false, "Invalid key"}
    json.NewEncoder(w).Encode(resp)
  }
}

func RootHandler(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
  w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hi there :)\n"))
}

func main() {
  //Load in data if it exist, if not create it.
  readFile("config", "config.data")
  readFile("workers", "workers.data")
  readFile("containers", "containers.data")

  //Temp functions to keep the files from being empty and unable to decode
  writeFile("config", "config.data")
  writeFile("workers", "workers.data")
  writeFile("containers", "containers.data")

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", RootHandler)

  router.HandleFunc("/containers/create", ContainersCreateHandler)
  router.HandleFunc("/containers/list/", ContainersListHandler)
  router.HandleFunc("/containers/list/{worker}", RootHandler)
  router.HandleFunc("/containers/stop/{container}", RootHandler)
  router.HandleFunc("/containers/status/{container}", RootHandler)
  router.HandleFunc("/containers/inspect/{container}", RootHandler)

  router.HandleFunc("/nodes/list", RootHandler)
  router.HandleFunc("/nodes/list/{type}", RootHandler)
  router.HandleFunc("/nodes/join", NodeJoinHandler)

  router.HandleFunc("/service/create", RootHandler)
  router.HandleFunc("/service/list", RootHandler)
  router.HandleFunc("/service/inspect", RootHandler)

  go controllerWorkers()
  go controllerContainers()

	handler := cors.Default().Handler(router)
	err := http.ListenAndServe(":8181", handler)
  log.Fatal(err)
}
