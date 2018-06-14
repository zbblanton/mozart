package main

import(
  "fmt"
  "time"
  "encoding/json"
  "bytes"
)

func containerControllerQueue(messages chan ControllerMsg) {
  //Set a ticker for a small delay (may not be needed for this queue)
  //Range through the messages, running executor on each
  //if it fails, add the retry queue
  ticker := time.NewTicker(time.Second)
  for message := range messages {
      //message := message.(test)
      //fmt.Println("Message", message.test2, time.Now())
      if(!containerControllerExecutor(message)){
        message.Retries++
        containerRetryQueue <- message
      }
      <- ticker.C
  }
}

func containerControllerRetryQueue(messages chan ControllerMsg) {
  //Set a ticker for a retry delay (careful, make sure the delay is what you want)
  //Range through the messages, running executor on each
  //if it fails, add to the retry queue again
  ticker := time.NewTicker(5 * time.Second)
  for message := range messages {
      if(!containerControllerExecutor(message)){
        message.Retries++
        containerRetryQueue <- message
      }
      <- ticker.C
  }
}

func containerControllerUpdateStateWithoutMux(containerName, state, serverIp string) {
  //Save new state
  container := containers.Containers[containerName]
  container.State = state
  fmt.Printf("Updating State for %s: State: %s Desired State: %s Current State: %s\n", container.Name, container.State, container.DesiredState, state)
  containers.Containers[containerName] = container

  //Send new state to master
  type StateUpdateReq struct {
    Key string
    ContainerName string
    State string
  }
  j := StateUpdateReq{Key: "ADDCHECKINGFORTHIS", ContainerName: containerName, State: state}
  b := new(bytes.Buffer)
  json.NewEncoder(b).Encode(j)
  url := "https://" + serverIp + ":47433/containers/" + containerName + "/state/update"
  _, err := callSecuredServer(agentTlsCert, agentTlsKey, caTLSCert, "POST", url, b)
  //_, err := http.Post(url, "application/json; charset=utf-8", b)
  if err != nil {
      panic(err)
  }
}


func containerControllerUpdateStateWithMux(containerName, state, serverIp string) {
  //Save new state
  containers.mux.Lock()
  container := containers.Containers[containerName]
  container.State = state
  fmt.Printf("Updating State for %s: State: %s Desired State: %s Current State: %s\n", container.Name, container.State, container.DesiredState, state)
  containers.Containers[containerName] = container
  containers.mux.Unlock()

  //Send new state to master
  type StateUpdateReq struct {
    Key string
    ContainerName string
    State string
  }
  j := StateUpdateReq{Key: "ADDCHECKINGFORTHIS", ContainerName: containerName, State: state}
  b := new(bytes.Buffer)
  json.NewEncoder(b).Encode(j)
  url := "https://" + serverIp + ":47433/containers/" + containerName + "/state/update"
  _, err := callSecuredServer(agentTlsCert, agentTlsKey, caTLSCert, "POST", url, b)
  //_, err := http.Post(url, "application/json; charset=utf-8", b)
  if err != nil {
      panic(err)
  }
}

func containerControllerExecutor(msg ControllerMsg) bool{
  //Case for each command, run the function matching the command and struct type
  //switch msg.(type) {
  switch msg.Action {
    case "create":
      msg := msg.Data.(Container)
      //Save container
      containers.mux.Lock()
      containers.Containers[msg.Name] = msg
      containers.mux.Unlock()
      //Convert container
      dockerContainer := ConvertContainerConfigToDockerContainerConfig(msg.Config)
      id, _ := DockerCreateContainer(msg.Name, dockerContainer)
      fmt.Print(id)
      DockerStartContainer(id)
      containerControllerUpdateStateWithMux(msg.Name, "running", *serverPtr)
      //containerControllerUpdateState(msg.Name, "running", *serverPtr)
      return true
    case "recreate":
      //msg := msg.Data.(Container)
      container := msg.Data.(Container)
      if(msg.Retries < 3){
        dockerContainer := ConvertContainerConfigToDockerContainerConfig(container.Config)
        id, _ := DockerCreateContainer(container.Name, dockerContainer)
        fmt.Print(id)
        DockerStartContainer(id)
        containerControllerUpdateStateWithMux(container.Name, "running", *serverPtr)
        return true
      }
      //send reschedule to master and delete this container from the map.
    case "stop":
      msg := msg.Data.(string)
      containers.mux.Lock()
      container := containers.Containers[msg]
      container.State = "stopping"
      container.DesiredState = "stopped"
      containers.Containers[msg] = container
      containers.mux.Unlock()
      err := DockerStopContainer(msg)
      if err != nil {
        return false
      }
      //Send state update to master
      containerControllerUpdateStateWithMux(msg, "stopped", *serverPtr)
      containers.mux.Lock()
      //Remove the container from the map
      delete(containers.Containers, msg)
      containers.mux.Unlock()
      return true
    default:
      return false
  }

  return true
}
