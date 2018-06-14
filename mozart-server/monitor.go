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
      fmt.Println("Checking Worker " + index + ".")
      if(checkWorkerHealth(worker.AgentIp, worker.AgentPort)){
        fmt.Println("Worker " + index + " is UP.")
      } else {
        fmt.Println("Worker " + index + " is DOWN.")
        worker.Status = "reconnecting"
        workers.Workers[index] = worker
      }
      //fmt.Print(worker)
    }
    workers.mux.Unlock()
    fmt.Println("Waiting 30 seconds!")
    time.Sleep(time.Duration(30) * time.Second)
  }
  os.Exit(1) //In case the for loop exits, stop the whole program.
}
