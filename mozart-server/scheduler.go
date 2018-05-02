package main

import "errors"

//Used to schedule actions such as creating or deleting a container
func schedulerCreateContainer(c ContainerConfig) error {
  worker, err := selectWorker()
  if err != nil {
    return err
  }
  newContainer := Container{
    Name: c.Name,
    State: "",
    DesiredState: "running",
    Config: c,
    Worker: worker.AgentIp}

  containers.mux.Lock()
  //config.Containers = append(config.Containers, newContainer)
  containers.Containers[c.Name] = newContainer
  writeFile("containers", "containers.data")
  containers.mux.Unlock()

  return nil
}

func schedulerStopContainer(containerName string) (err error){
  containers.mux.Lock()
  if _, ok := containers.Containers[containerName]; !ok {
    err = errors.New("Container does not exist.")
  } else {
    container := containers.Containers[containerName]
    container.DesiredState = "stopped"
    containers.Containers[containerName] = container
    writeFile("containers", "containers.data")
  }

  containers.mux.Unlock()

  return err
}
