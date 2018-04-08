package main

import(
  "os"
  "fmt"
  "time"
  "bytes"
  "encoding/json"
  "net/http"
)

func controllerContainersStart(c Container){
  //Will need to add support for the worker key!!!!!
  type CreateReq struct {
    Key string
    Container ContainerConfig
  }

  j := CreateReq{Key: "NEEDTOADDSUPPORTFORTHIS!!!", Container: c.Config}

  b := new(bytes.Buffer)
  json.NewEncoder(b).Encode(j)
  url := "http://" + c.Worker + ":8080" + "/create"
  _, err := http.Post(url, "application/json; charset=utf-8", b)
  if err != nil {
      panic(err)
  }
}

func controllerContainers() {
  //TODO: We need to add an initializing part so that we can get get
  //containers statuses before we start looping.
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
