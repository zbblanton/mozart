package main

import(
  "os"
  "log"
  "net/http"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
  "crypto/rand"
  "crypto/tls"
  "crypto/x509"
	"encoding/base64"
  "encoding/json"
  "fmt"
  "io/ioutil"
)

func RootHandler(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
  w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hi there :)\n"))
}

func NodeInitialJoinHandler(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json; charset=UTF-8")
  w.WriteHeader(http.StatusOK)
  defer r.Body.Close()

  j := NodeInitialJoinReq{}
  json.NewDecoder(r.Body).Decode(&j)

  //Verify key
  if(j.JoinKey != config.AgentJoinKey){
    fmt.Println(config.AgentJoinKey)
    fmt.Println(j.JoinKey)
    resp := NodeJoinResp{ServerKey: "", Success: false, Error: "Invalid join key"}
    json.NewEncoder(w).Encode(resp)
    return
  }

  //Decode the CSR from base64
  csr, err := base64.URLEncoding.DecodeString(j.Csr)
  if err != nil {
      panic(err)
  }

  //Sign the CSR
  signedCert, err := signCSR(config.CaCert, config.CaKey, csr, j.AgentIp)

  //Prepare signed cert to be sent to agent
  signedCertString := base64.URLEncoding.EncodeToString(signedCert)

  //Prepare CA to be sent to agent
  ca, err := ioutil.ReadFile(config.CaCert)
  if err != nil {
    panic("cant open file")
  }
  caString := base64.URLEncoding.EncodeToString(ca)

  resp := NodeInitialJoinResp{caString, signedCertString, true, ""}
  json.NewEncoder(w).Encode(resp)
}

func NodeJoinHandler(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json; charset=UTF-8")
  w.WriteHeader(http.StatusOK)
  defer r.Body.Close()

  j := NodeJoinReq{}
  json.NewDecoder(r.Body).Decode(&j)

  //ADD VERIFICATION!!!!

  //Verify key
  if(j.JoinKey != config.AgentJoinKey){
    fmt.Println(config.AgentJoinKey)
    fmt.Println(j.JoinKey)
    resp := NodeJoinResp{ServerKey: "", Success: false, Error: "Invalid join key"}
    json.NewEncoder(w).Encode(resp)
    return
  }

  //Check if worker exist and if it has an active or maintenance status
  if worker, ok := workers.Workers[j.AgentIp]; ok {
    if(worker.Status == "connected" || worker.Status == "maintenance"){
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
  newWorker := Worker{AgentIp: j.AgentIp, AgentPort: "49433", ServerKey: serverKey, AgentKey: j.AgentKey, Status: "active"}
  workers.mux.Lock()
  workers.Workers[j.AgentIp] = newWorker
  writeFile("workers", "workers.data")
  workers.mux.Unlock()

  //Send containers and key to worker
  workerContainers := make(map[string]Container)
  containers.mux.Lock()
  for _, container := range containers.Containers {
    if container.Worker == j.AgentIp {
      workerContainers[container.Name] = container
    }
  }
  containers.mux.Unlock()
  resp := NodeJoinResp{ServerKey: serverKey, Containers: workerContainers, Success: true, Error: ""}
  json.NewEncoder(w).Encode(resp)
}

func ContainersCreateHandler(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json; charset=UTF-8")
  w.WriteHeader(http.StatusOK)
  defer r.Body.Close()

  j := ContainerConfig{}
  json.NewDecoder(r.Body).Decode(&j)
  if(ContainersCreateVerification(j)){
    fmt.Println("Received a run request for config: ", j, "adding to queue.")
    containerQueue <- j
      /*
      err := schedulerCreateContainer(j)
      if err != nil {
        resp := Resp{false, "No workers!"} //Add better error.
        json.NewEncoder(w).Encode(resp)
        return
      }*/
      resp := Resp{true, ""}
      json.NewEncoder(w).Encode(resp)
  }else {
      resp := Resp{false, "Invalid data"} //Add better error.
      json.NewEncoder(w).Encode(resp)
  }
}

func ContainersStopHandler(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json; charset=UTF-8")
  w.WriteHeader(http.StatusOK)
  defer r.Body.Close()

  vars := mux.Vars(r)
  containerName := vars["container"]
  if(containerName == ""){
    resp := Resp{false, "Must provide a container name."}
    json.NewEncoder(w).Encode(resp)
    return
  }

  //Check if container exist
  containers.mux.Lock()
  if _, ok := containers.Containers[containerName]; !ok {
    resp := Resp{false, "Cannot find container"}
    json.NewEncoder(w).Encode(resp)
  } else {
    resp := Resp{true, ""}
    json.NewEncoder(w).Encode(resp)
    //Add to queue
    containerQueue <- containerName
  }
  containers.mux.Unlock()

  /*
  err := schedulerStopContainer(containerName)
  if err != nil {
    resp := Resp{false, "Cannot find container"}
    json.NewEncoder(w).Encode(resp)
    return
  }*/

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
  if j.State == "stopped" && containers.Containers[j.ContainerName].DesiredState == "stopped" {
    delete(containers.Containers, j.ContainerName)
  } else {
    c := containers.Containers[j.ContainerName]
    c.State = j.State
    fmt.Print(c)
    containers.Containers[j.ContainerName] = c
  }
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
    Containers []Container
    Success bool
    Error string
  }

  c := ContainersListWorkers{[]Container{}, true, ""}
  for _, container := range containers.Containers {
    if (container.Worker == vars["worker"]){
      c.Containers = append(c.Containers, container)
    }
  }

  resp := c
  json.NewEncoder(w).Encode(resp)
}

func NodeListHandler(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json; charset=UTF-8")
  w.WriteHeader(http.StatusOK)
  defer r.Body.Close()

  resp := NodeListResp{workers.Workers, true, ""}
  json.NewEncoder(w).Encode(resp)
}

func startJoinServer(serverIp string, joinPort string, caCert string, serverCert string, serverKey string){
  router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", NodeInitialJoinHandler)
  handler := cors.Default().Handler(router)

  //Setup server config
  server := &http.Server{
        Addr: serverIp + ":" + joinPort,
        Handler: handler}

  //Start Join server
  fmt.Println("Starting join server...")
  err := server.ListenAndServeTLS(serverCert, serverKey)
  log.Fatal(err)
}

func startApiServer(serverIp string, serverPort string, caCert string, serverCert string, serverKey string) {
  router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", RootHandler)

  router.HandleFunc("/containers/create", ContainersCreateHandler)
  router.HandleFunc("/containers/stop/{container}", ContainersStopHandler)
  router.HandleFunc("/containers/list", ContainersListHandler)
  router.HandleFunc("/containers/list/{worker}", ContainersListWorkersHandler)
  router.HandleFunc("/containers/{container}/state/update", ContainersStateUpdateHandler)
  router.HandleFunc("/containers/status/{container}", RootHandler)
  router.HandleFunc("/containers/inspect/{container}", RootHandler)

  router.HandleFunc("/nodes/list", NodeListHandler)
  router.HandleFunc("/nodes/list/{type}", RootHandler)
  router.HandleFunc("/nodes/join", NodeJoinHandler)

  router.HandleFunc("/service/create", RootHandler)
  router.HandleFunc("/service/list", RootHandler)
  router.HandleFunc("/service/inspect", RootHandler)

  handler := cors.Default().Handler(router)

  //Setup TLS config
  rootCa, err := ioutil.ReadFile(caCert)
  if err != nil {
    panic("cant open file")
  }
  rootCaPool := x509.NewCertPool()
  if ok := rootCaPool.AppendCertsFromPEM([]byte(rootCa)); !ok {
    panic("Cannot parse root CA.")
  }
  tlsCfg := &tls.Config{
      RootCAs: rootCaPool,
      ClientCAs: rootCaPool,
      ClientAuth: tls.RequireAndVerifyClientCert}

  //Setup server config
  server := &http.Server{
        Addr: serverIp + ":" + serverPort,
        Handler: handler,
        TLSConfig: tlsCfg}


  //Start API server
  err = server.ListenAndServeTLS(serverCert, serverKey)
	//handler := cors.Default().Handler(router)
  //err = http.ListenAndServe(ServerIp + ":" + ServerPort, handler)
  log.Fatal(err)
}
