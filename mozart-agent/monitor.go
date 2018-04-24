package main

import(
  "os"
  "bytes"
  "fmt"
  "time"
	"encoding/json"
	"net/http"
)

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
