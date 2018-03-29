package main

import(
  "os"
	"os/exec"
  "net"
  "bytes"
  "fmt"
  //"io/ioutil"
	"log"
	"strings"
	"encoding/json"
	"net/http"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

type ContainerHostConfigMounts struct {
  Target string
	Source string
	Type string
	ReadOnly bool
}

type ContainerHostConfigPortBindings struct {
  HostIp string
  HostPort string
}

type ContainerHostConfig struct {
  PortBindings map[string][]ContainerHostConfigPortBindings
  Mounts []ContainerHostConfigMounts
  AutoRemove bool
  Privileged bool
}

type ContainerConfig struct {
  Image string
  Env []string
  ExposedPorts map[string]struct{}
  HostConfig ContainerHostConfig
}

type CreateReq struct {
  Key string
  ContainerName string
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

func CreateContainer(ContainerName string, Container ContainerConfig) (id string, err error){
  tr := &http.Transport{
    Dial: fakeDial,
  }

  client := &http.Client{Transport: tr}
  b := new(bytes.Buffer)
  json.NewEncoder(b).Encode(Container)
  url := "http://d/containers/create"
  if(ContainerName != ""){
    url = url + "?name=" + ContainerName
  }
  fmt.Println(url)
  req, err := http.NewRequest("POST", url, b)
  req.Header.Set("Content-Type", "application/json")
  resp, err := client.Do(req)
  if err != nil {
      panic(err)
  }

  type ContainerCreateResp struct {
    Id string
    Warnings string
    Message string
  }
  j := ContainerCreateResp{}
	json.NewDecoder(resp.Body).Decode(&j)
  resp.Body.Close()

  //ADD VERIFICATION HERE!!!!!!!!!!!!!

  return j.Id, nil
}

func StartContainer(ContainerId string) error{
  tr := &http.Transport{
    Dial: fakeDial,
  }

  client := &http.Client{Transport: tr}
  url := "http://d/containers/" + ContainerId + "/start"
  req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(`{	}`)))
  req.Header.Set("Content-Type", "application/json")
  resp, err := client.Do(req)
  if err != nil {
      panic(err)
  }

  type ContainerStartResp struct {
    Message string
  }
  j := ContainerStartResp{}
	json.NewDecoder(resp.Body).Decode(&j)
  resp.Body.Close()

  //ADD VERIFICATION HERE!!!!!!!!!!!!!

  return nil
}

func StopContainer(ContainerId string) error{
  tr := &http.Transport{
    Dial: fakeDial,
  }

  client := &http.Client{Transport: tr}
  url := "http://d/containers/" + ContainerId + "/stop"
  req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(`{	}`)))
  req.Header.Set("Content-Type", "application/json")
  resp, err := client.Do(req)
  if err != nil {
      panic(err)
  }

  type ContainerStopResp struct {
    Message string
  }
  j := ContainerStopResp{}
	json.NewDecoder(resp.Body).Decode(&j)
  resp.Body.Close()

  //ADD VERIFICATION HERE!!!!!!!!!!!!!

  return nil
}

func CreateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	defer r.Body.Close()

  //Read json in and decode it
	j := CreateReq{}
	json.NewDecoder(r.Body).Decode(&j)

  id, _ := CreateContainer(j.ContainerName, j.Container)
  StartContainer(id)

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
    StopContainer(containerName)
    p := Resp{true, ""}
    json.NewEncoder(w).Encode(p)
  } else {
    p := Resp{false, "Must provide a container name or ID."}
    json.NewEncoder(w).Encode(p)
  }
}

func main() {
	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/", RootHandler)
  router.HandleFunc("/create", CreateHandler)
  router.HandleFunc("/list/", RootHandler)
  router.HandleFunc("/stop/{container}", StopHandler)
  router.HandleFunc("/status/{container}", RootHandler)
  router.HandleFunc("/inspect/{container}", RootHandler)

	handler := cors.Default().Handler(router)
	err := http.ListenAndServe(":8080", handler)
  log.Fatal(err)
}
