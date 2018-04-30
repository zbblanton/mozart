package main

import(
  "os"
  "bytes"
  "fmt"
  "time"
	"encoding/json"
)

func MonitorContainers(serverIp, agentIp string) {
  for {
    //Get list of containers that should be running on this worker from the master
    url := "https://" + serverIp + ":47433/containers/list/" + agentIp
    req, err := callSecuredServer(agentTlsCert, agentTlsKey, caTLSCert, "GET", url, nil)
    if err != nil {
  		panic(err)
  	}
    /*req, err := http.Get(url)
    if err != nil {
        panic(err)
    }*/

    type Container struct {
      Name string
      State string
      DesiredState string
      Config ContainerConfig
      Worker string
    }

    type ContainersListResp struct {
      Containers []Container
      Success bool
      Error string
    }
    j := ContainersListResp{}
    //json.NewDecoder(req.Body).Decode(&j)
    err = json.Unmarshal(req, &j)
  	if err != nil {
  		fmt.Println("error:", err)
  	}
    //req.Body.Close()

    //fmt.Print(j)
    //Loop through the containers and check the status, if not running send new state to master
    for _, container := range j.Containers {
      //fmt.Println(container)
      status, err := DockerContainerStatus(container.Name)
      if err != nil{
        panic("Failed to get container status.")
      }
      if (status != "running" && container.State != "" && container.DesiredState != "stopped") {
        fmt.Println(container.Name + ": Not running, notifying master.")
        type StateUpdateReq struct {
          Key string
          ContainerName string
          State string
        }
        j := StateUpdateReq{Key: "ADDCHECKINGFORTHIS", ContainerName: container.Name, State: status}
        b := new(bytes.Buffer)
        json.NewEncoder(b).Encode(j)
        url := "https://" + serverIp + ":47433/containers/" + container.Name + "/state/update"
        _, err := callSecuredServer(agentTlsCert, agentTlsKey, caTLSCert, "POST", url, b)
        //_, err := http.Post(url, "application/json; charset=utf-8", b)
        if err != nil {
            panic(err)
        }
      }
      //fmt.Println(status)
    }

    fmt.Println("Waiting 5 seconds!")
    time.Sleep(time.Duration(5) * time.Second)
  }
  os.Exit(1) //In case the for loop exits, stop the whole program.
}
