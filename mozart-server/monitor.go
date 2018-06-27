package main

import(
  "os"
  "fmt"
  "time"
)

func monitorWorkers() {
  fmt.Println("Waiting 15 seconds before starting the worker controller!")
  time.Sleep(time.Duration(15) * time.Second)
  for {
    //Loop through workers and make sure the desiredState matches the state, if not, perform DesiredState action.
    workers.mux.Lock()
    for index, worker := range workers.Workers {
      //if(checkWorkerHealth(worker.AgentIp, worker.AgentPort)){
      //  fmt.Println("Worker " + index + " is UP.")
      //} else {
      if worker.Status != "reconnecting" && worker.Status != "disconnected"{
        fmt.Println("Checking Worker " + index + ".")
        if(!checkWorkerHealth(worker.AgentIp, worker.AgentPort)){
          fmt.Println("Worker " + index + " is DOWN.")
          //Need to add an update state function here for mux control like we have for container state
          worker.Status = "reconnecting"
          workers.Workers[index] = worker
          ///////////////////////////////////////////
          data := ControllerReconnectMsg{worker: worker, disconnectTime: time.Now()}
          q := ControllerMsg{Action: "reconnect", Data: data}
          workerQueue <- q
        }
      }
      //fmt.Print(worker)
    }
    workers.mux.Unlock()
    fmt.Println("Waiting 10 seconds!")
    time.Sleep(time.Duration(10) * time.Second)
  }
  os.Exit(1) //In case the for loop exits, stop the whole program.
}
