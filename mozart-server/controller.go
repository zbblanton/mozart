package main

import(
  "os"
  "fmt"
  "time"
  "bytes"
  "encoding/json"
  //"net/http"
)

//////////////////////////////////////////////////////////
func containerControllerQueue(messages chan interface{}) {
  //Set a ticker for a small delay (may not be needed for this queue)
  //Range through the messages, running executor on each
  //if it fails, add the retry queue
  ticker := time.NewTicker(time.Second)
  for message := range messages {
      //message := message.(test)
      //fmt.Println("Message", message.test2, time.Now())
      if(!containerControllerExecutor(message)){
        containerRetryQueue <- message
      }
      <- ticker.C
  }
}

func containerControllerRetryQueue(messages chan interface{}) {
  //Set a ticker for a retry delay (careful, make sure the delay is what you want)
  //Range through the messages, running executor on each
  //if it fails, add to the retry queue again
  ticker := time.NewTicker(5 * time.Second)
  for message := range messages {
      if(!containerControllerExecutor(message)){
        containerRetryQueue <- message
      }
      <- ticker.C
  }
}

func containerControllerExecutor(msg interface{}) bool{
  //Case for each command, run the function matching the command and struct type
  switch msg.(type) {
    case ContainerConfig:
      msg := msg.(ContainerConfig)
      return containerControllerStart(msg)
    case string:
      msg := msg.(string)
      return containerControllerStop(msg)
    default:
      return false
  }

  return true
}

func containerControllerStart(c ContainerConfig) bool {
  worker, err := selectWorker()
  if err != nil {
    fmt.Println("Error:", err)
    return false
  }
  newContainer := Container{
    Name: c.Name,
    State: "",
    DesiredState: "running",
    Config: c,
    Worker: worker.AgentIp}

  //Will need to add support for the worker key!!!!!
  type CreateReq struct {
    Key string
    Container Container
  }
  j := CreateReq{Key: "NEEDTOADDSUPPORTFORTHIS!!!", Container: newContainer}
  b := new(bytes.Buffer)
  json.NewEncoder(b).Encode(j)
  url := "https://" + newContainer.Worker + ":49433" + "/create"
  _, err = callSecuredAgent(serverTlsCert, serverTlsKey, caTlsCert, "POST", url, b)
  if err != nil {
		//panic(err)
    return false
	}

  //Save container
  containers.mux.Lock()
  //config.Containers = append(config.Containers, newContainer)
  containers.Containers[c.Name] = newContainer
  writeFile("containers", "containers.data")
  containers.mux.Unlock()

  return true
}

func containerControllerStop(name string) bool {
  //Update container desired state
  containers.mux.Lock()
  container := containers.Containers[name]
  container.DesiredState = "stopped"
  containers.Containers[name] = container
  writeFile("containers", "containers.data")
  containers.mux.Unlock()

  //Will need to add support for the worker key!!!!!
  url := "https://" + container.Worker + ":49433" + "/stop/" + container.Name
  _, err := callSecuredAgent(serverTlsCert, serverTlsKey, caTlsCert, "GET", url, nil)
  if err != nil {
		//panic(err)
    return false
	}

  return true
}

//////////////////////////////////////////////////////////






func controllerContainersStart(c Container){
  //Will need to add support for the worker key!!!!!
  type CreateReq struct {
    Key string
    Container ContainerConfig
  }

  j := CreateReq{Key: "NEEDTOADDSUPPORTFORTHIS!!!", Container: c.Config}

  b := new(bytes.Buffer)
  json.NewEncoder(b).Encode(j)
  url := "https://" + c.Worker + ":49433" + "/create"

  _, err := callSecuredAgent(serverTlsCert, serverTlsKey, caTlsCert, "POST", url, b)
  if err != nil {
		panic(err)
	}
}

func controllerContainersStop(c Container){
  //Will need to add support for the worker key!!!!!
  type CreateReq struct {
    Key string
    Container ContainerConfig
  }

  url := "https://" + c.Worker + ":49433" + "/stop/" + c.Name

  _, err := callSecuredAgent(serverTlsCert, serverTlsKey, caTlsCert, "GET", url, nil)
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
        } else if(container.DesiredState == "stopped"){
          //Run function to start a container
          //Below we assume that the containers actually start and put in a running state. Will need to add actual checks.
          controllerContainersStop(container)
          container.State = "stopped"
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
