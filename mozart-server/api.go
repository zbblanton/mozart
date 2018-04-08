package main

import(
  "os"
  "log"
  "net/http"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
  "crypto/rand"
	"encoding/base64"
  "encoding/json"
  "fmt"
)

func RootHandler(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
  w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hi there :)\n"))
}

func NodeJoinHandler(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json; charset=UTF-8")
  w.WriteHeader(http.StatusOK)
  defer r.Body.Close()

  j := NodeJoinReq{}
  json.NewDecoder(r.Body).Decode(&j)

  //ADD VERIFICATION!!!!

  //Verify key
  if(j.JoinKey != config.WorkerJoinKey){
    fmt.Println(config.WorkerJoinKey)
    fmt.Println(j.JoinKey)
    resp := NodeJoinResp{ServerKey: "", Success: false, Error: "Invalid join key"}
    json.NewEncoder(w).Encode(resp)
    return
  }

  //Check if worker exist and if it has an active or maintenance status
  if worker, ok := workers.Workers[j.AgentIp]; ok {
    if(worker.Status == "active" || worker.Status == "maintenance"){
      resp := NodeJoinResp{ServerKey: "", Success: false, Error: "Host already exist and has an active or maintenance status."}
      json.NewEncoder(w).Encode(resp)
      return
    }
  }

  //Generating key taken from http://blog.questionable.services/article/generating-secure-random-numbers-crypto-rand/
  //Generate random key
  randKey := make([]byte, 128)
  _, err := rand.Read(randKey)
  if err != nil {
    fmt.Println("Error generating a new worker key, we are going to exit here due to possible system errors.")
    os.Exit(1)
  }
  serverKey := base64.URLEncoding.EncodeToString(randKey)
  //Save key to config
  newWorker := Worker{AgentIp: j.AgentIp, AgentPort: j.AgentPort, ServerKey: serverKey, AgentKey: j.AgentKey, Status: "active"}
  workers.mux.Lock()
  workers.Workers[j.AgentIp] = newWorker
  writeFile("workers", "workers.data")
  workers.mux.Unlock()
  //Send key to worker
  resp := NodeJoinResp{ServerKey: serverKey, Success: true, Error: ""}
  json.NewEncoder(w).Encode(resp)
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

func ContainersStateUpdateHandler(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json; charset=UTF-8")
  w.WriteHeader(http.StatusOK)
  defer r.Body.Close()

  type StateUpdateReq struct {
    Key string
    ContainerName string
    State string
  }

  j := StateUpdateReq{}
	json.NewDecoder(r.Body).Decode(&j)

  //TODO: Verify Worker Key here, the container must live on this host.
  containers.mux.Lock()
  fmt.Print(j)
  c := containers.Containers[j.ContainerName]
  c.State = j.State
  fmt.Print(c)
  containers.Containers[j.Key] = c
  containers.mux.Unlock()

  resp := Resp{true, ""}
  json.NewEncoder(w).Encode(resp)
}

func ContainersListHandler(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json; charset=UTF-8")
  w.WriteHeader(http.StatusOK)
  defer r.Body.Close()

  resp := ContainerListResp{containers.Containers, true, ""}
  json.NewEncoder(w).Encode(resp)
}

func ContainersListWorkersHandler(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json; charset=UTF-8")
  w.WriteHeader(http.StatusOK)
  vars := mux.Vars(r)
  defer r.Body.Close()

  type ContainersListWorkers struct {
    Containers []string
    Success bool
    Error string
  }

  c := ContainersListWorkers{[]string{}, true, ""}
  for _, container := range containers.Containers {
    if (container.Worker == vars["worker"]){
      c.Containers = append(c.Containers, container.Name)
    }
  }

  resp := c
  json.NewEncoder(w).Encode(resp)
}

func startApiServer(ServerIp string, ServerPort string) {
  router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", RootHandler)

  router.HandleFunc("/containers/create", ContainersCreateHandler)
  router.HandleFunc("/containers/stop/{container}", RootHandler)
  router.HandleFunc("/containers/list", ContainersListHandler)
  router.HandleFunc("/containers/list/{worker}", ContainersListWorkersHandler)
  router.HandleFunc("/containers/{container}/state/update", ContainersStateUpdateHandler)
  router.HandleFunc("/containers/status/{container}", RootHandler)
  router.HandleFunc("/containers/inspect/{container}", RootHandler)

  router.HandleFunc("/nodes/list", RootHandler)
  router.HandleFunc("/nodes/list/{type}", RootHandler)
  router.HandleFunc("/nodes/join", NodeJoinHandler)

  router.HandleFunc("/service/create", RootHandler)
  router.HandleFunc("/service/list", RootHandler)
  router.HandleFunc("/service/inspect", RootHandler)

  //Start API server
	handler := cors.Default().Handler(router)
	err := http.ListenAndServe(ServerIp + ":" + ServerPort, handler)
  log.Fatal(err)
}
