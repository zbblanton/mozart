package main

import(
  "fmt"
  "time"
)

//MonitorContainers - Monitors the containers on the host for status changes
func MonitorContainers(serverIP, agentIP string) {
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
        if container.State != "starting" && container.State != "stopping" {
          //containerControllerUpdateState(container.Name, state, serverIp)
          switch container.DesiredState {
            case "running":
              containerControllerUpdateStateWithoutMux(container.Name, "restarting", serverIP)
              q := ControllerMsg{Action: "recreate", Data: container}
              containerQueue <- q
            default:
          }
        }
      }
    }
    containers.mux.Unlock()

    fmt.Println("Waiting 10 seconds!")
    //time.Sleep(time.Duration(5) * time.Second)
    <- ticker.C
  }
  //os.Exit(1) //In case the for loop exits, stop the whole program.//This is unreachable
}
