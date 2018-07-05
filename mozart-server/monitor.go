package main

import(
  "os"
  "fmt"
  "time"
  "encoding/json"
)

func monitorWorkers() {
  fmt.Println("Waiting 15 seconds before starting the worker controller!")
  time.Sleep(time.Duration(15) * time.Second)
  for {
    //Create worker map
    workers := make(map[string]Worker)

    //Get all workers
    dataBytes, _ := ds.GetByPrefix("mozart/workers")
    for k, v := range dataBytes {
      var data Worker
      err := json.Unmarshal(v, &data)
      if err != nil {
        panic(err)
      }
      workers[k] = data
    }

    //Loop through workers and make sure the desiredState matches the state, if not, perform DesiredState action.
    //workers.mux.Lock()
    for index, worker := range workers {
      //if(checkWorkerHealth(worker.AgentIp, worker.AgentPort)){
      //  fmt.Println("Worker " + index + " is UP.")
      //} else {
      if worker.Status != "reconnecting" && worker.Status != "disconnected"{
        fmt.Println("Checking Worker " + index + ".")
        if(!checkWorkerHealth(worker.AgentIp, worker.AgentPort)){
          fmt.Println("Worker " + index + " is DOWN.")
          //Need to add an update state function here for mux control like we have for container state
          worker.Status = "reconnecting"
          //workers[index] = worker
          b, err := json.Marshal(worker)
          if err != nil {
            panic(err)
          }
          ds.Put(index, b)
          ///////////////////////////////////////////
          data := ControllerReconnectMsg{worker: worker, disconnectTime: time.Now()}
          q := ControllerMsg{Action: "reconnect", Data: data}
          workerQueue <- q
        }
      }
      //fmt.Print(worker)
    }
    //workers.mux.Unlock()
    fmt.Println("Waiting 10 seconds!")
    time.Sleep(time.Duration(10) * time.Second)
  }
  os.Exit(1) //In case the for loop exits, stop the whole program.
}
