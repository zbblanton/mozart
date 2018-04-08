package main

//Used to schedule actions such as creating or deleting a container
func schedulerCreateContainer(c ContainerConfig) {
  worker := selectWorker()
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
}
