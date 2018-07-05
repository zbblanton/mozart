package main

import (
    "fmt"
    "encoding/json"
    "errors"
  )

func selectWorker() (w Worker, err error) {
  //Create maps
  workers := make(map[string]Worker)
  containers := make(map[string]Container)

  //Get all workers
  dataBytes, _ := ds.GetByPrefix("mozart/workers")
  for k, v := range dataBytes {
    var data Worker
    err = json.Unmarshal(v, &data)
    if err != nil {
      panic(err)
    }
    workers[k] = data
  }

  if(len(workers) == 0){
    return Worker{}, errors.New("No workers!")
  }

  fmt.Println("List of workers to consider:", workers)

  workerPool := make(map[string]uint)

  //Add all workers that are active to the worker pool.
  for _, worker := range workers {
    if worker.Status == "connected" || worker.Status == "active" {
      workerPool[worker.AgentIp] = 0
    }
  }

  if(len(workerPool) == 0){
    return Worker{}, errors.New("No active workers!")
  }

  //Get containers
  dataBytes, _ = ds.GetByPrefix("mozart/containers")
  for k, v := range dataBytes {
    var data Container
    err = json.Unmarshal(v, &data)
    if err != nil {
      panic(err)
    }
    containers[k] = data
  }

  //Scan containers how many containers each worker is hosting
  for _, container := range containers {
    if _, ok := workerPool[container.Worker]; ok {
      curr := workerPool[container.Worker]
      curr = curr + 1
      workerPool[container.Worker] = curr
    }
  }

  //Find the lowest used worker
  firstRun := true
  lowestWorker := ""
  var lowestContainers uint = 0
  for workerIp, numContainers := range workerPool {
    //If a worker in the pool has no containers, return it.
    if numContainers == 0 {
      fmt.Println("First container so Worker", workerIp, "selected.")
      return workers["mozart/workers/" + workerIp], nil
    }

    if(firstRun){
      firstRun = false
      lowestContainers = numContainers
      lowestWorker = workerIp
    }

    if(numContainers < lowestContainers) {
      lowestWorker = workerIp
      lowestContainers = numContainers
    }
  }

  fmt.Println("Worker", lowestWorker,"selected.")
  return workers["mozart/workers/" + lowestWorker], nil
}
