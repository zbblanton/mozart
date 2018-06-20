package main

import(
  "os"
  "fmt"
  "time"
)

func MonitorContainers(serverIp, agentIp string) {
  ticker := time.NewTicker(10 * time.Second)
  for {
    //Loop through the containers and check the status, if not running send new state to master
    containers.mux.Lock()
    for _, container := range containers.Containers {
      fmt.Println("Checking:", container)
      state, err := DockerContainerStatus(container.Name)
      if err != nil{
        panic("Failed to get container status.")
      }

      if(container.State != state || container.DesiredState != state){
        if container.State != "starting" || container.State != "stopping" {
          //containerControllerUpdateState(container.Name, state, serverIp)
          switch container.DesiredState {
            case "running":
              containerControllerUpdateStateWithoutMux(container.Name, "restarting", serverIp)
              q := ControllerMsg{Action: "recreate", Data: container}
              containerQueue <- q
            default:
          }
        }
      }

      /*
      if(state == "") {
        switch container.DesiredState {
          case "running":
            state = "starting"
          case "stopped":
            state = "stopped"
        }
      }

      //if (state != "running" && container.State != "" && container.DesiredState != "stopped") {
      if(container.State == "" && state == "running"){
        containerControllerUpdateState(container.Name, state, serverIp)
        //fmt.Printf("Updating State for %s: State: %s Desired State: %s Current State: %s\n", container.Name, container.State, container.DesiredState, state)
      } else if (state != "running" && container.DesiredState != "stopped"){
        containerControllerUpdateState(container.Name, state, serverIp)
        //fmt.Printf("Updating State for %s: State: %s Desired State: %s Current State: %s\n", container.Name, container.State, container.DesiredState, state)
      } else if (container.State != state && container.State != ""){
        containerControllerUpdateState(container.Name, state, serverIp)
        //fmt.Printf("Updating State for %s: State: %s Desired State: %s Current State: %s\n", container.Name, container.State, container.DesiredState, state)
        switch state {
          case "stopped":
            q := ControllerMsg{Action: "recreate", Data: container}
            containerQueue <- q
        }
      }
      */
      /*if (state != "running" && container.State != "" && container.DesiredState != "stopped") {
        fmt.Println(container.Name + ": Not running, notifying master.")
        containerControllerUpdateState(container.Name, state, serverIp)
      }*/
      //fmt.Println(status)
    }
    containers.mux.Unlock()

    fmt.Println("Waiting 10 seconds!")
    //time.Sleep(time.Duration(5) * time.Second)
    <- ticker.C
  }
  os.Exit(1) //In case the for loop exits, stop the whole program.
}
