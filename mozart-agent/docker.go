package main

import(
  "io"
  "io/ioutil"
  "bufio"
  "bytes"
	"encoding/json"
	"net/http"
  "fmt"
)

func ConvertContainerConfigToDockerContainerConfig(c ContainerConfig) DockerContainerConfig {
  d := DockerContainerConfig{}
  d.Image = c.Image
  d.Env = c.Env
  d.Labels = make(map[string]string)
  d.Labels["mozart"] = "true"
  d.ExposedPorts = make(map[string]struct{})
  for _, port := range c.ExposedPorts {
    d.ExposedPorts[port.ContainerPort + "/tcp"] = struct{}{}
  }
  d.HostConfig.PortBindings = make(map[string][]DockerContainerHostConfigPortBindings)
  for _, port := range c.ExposedPorts {
    p := DockerContainerHostConfigPortBindings{}
    p.HostIp = port.HostIp
    p.HostPort = port.HostPort
    d.HostConfig.PortBindings[port.ContainerPort + "/tcp"] = []DockerContainerHostConfigPortBindings{p}
  }
  d.HostConfig.Mounts = c.Mounts
  d.HostConfig.AutoRemove = c.AutoRemove
  d.HostConfig.Privileged = c.Privileged

  return d
}

func DockerCallRuntimeApi(method string, url string, body io.Reader) (respBody []byte, err error)  {
  tr := &http.Transport{
    Dial: fakeDial,
  }

  client := &http.Client{Transport: tr}
  b := new(bytes.Buffer) //I dont think we need this line
  json.NewEncoder(b).Encode(body) //and this line, If you look the functions that call this function already encode the body...
  req, err := http.NewRequest(method, url, body)
  req.Header.Set("Content-Type", "application/json")
  resp, err := client.Do(req)
  if err != nil {
      panic(err)
  }

  reader := bufio.NewReader(resp.Body)
  respBody, _ = ioutil.ReadAll(reader)

  resp.Body.Close()

  return respBody, nil
}

func DockerCreateContainer(ContainerName string, Container DockerContainerConfig) (id string, err error){
  buff := new(bytes.Buffer)
  json.NewEncoder(buff).Encode(Container)
  url := "http://d/containers/create"
  if(ContainerName != ""){
    url = url + "?name=" + ContainerName
  }

  body, _ := DockerCallRuntimeApi("POST", url, buff)

  type ContainerCreateResp struct {
    Id string
    Warnings string
    Message string
  }
  j := ContainerCreateResp{}
  b := bytes.NewReader(body)
  json.NewDecoder(b).Decode(&j)

  fmt.Println("Response from Docker Runtime API:", j)

  //ADD VERIFICATION HERE!!!!!!!!!!!!!

  return j.Id, nil
}

func DockerList() (containerList []string, err error) {
  url := "http://d/containers/" + "/json"
  body, _ := DockerCallRuntimeApi("GET", url, nil)
  type DockerListResp struct {
    List []struct {
      Id string
    }
  }
  j := DockerListResp{}
  b := bytes.NewReader(body)
	json.NewDecoder(b).Decode(&j)

  //ADD VERIFICATION HERE!!!!!!!!!!!!!

  for _, container := range j.List {
    containerList = append(containerList, container.Id)
  }

  return containerList, nil
}

func DockerGetId(ContainerName string) (Id string, err error) {
  url := "http://d/containers/" + ContainerName + "/json"
  body, _ := DockerCallRuntimeApi("GET", url, nil)
  type DockerStatusResp struct {
    Id string
  }
  j := DockerStatusResp{}
  b := bytes.NewReader(body)
	json.NewDecoder(b).Decode(&j)
  //ADD VERIFICATION HERE!!!!!!!!!!!!!

  return j.Id, nil
}

func DockerStartContainer(ContainerId string) error{
  url := "http://d/containers/" + ContainerId + "/start"
  body, _ := DockerCallRuntimeApi("POST", url, bytes.NewBuffer([]byte(`{	}`)))
  type ContainerStartResp struct {
    Message string
  }
  j := ContainerStartResp{}
  b := bytes.NewReader(body)
	json.NewDecoder(b).Decode(&j)

  //ADD VERIFICATION HERE!!!!!!!!!!!!!

  return nil
}

func DockerStopContainer(ContainerId string) error{
  url := "http://d/containers/" + ContainerId + "/stop"
  body, _ := DockerCallRuntimeApi("POST", url, bytes.NewBuffer([]byte(`{	}`)))
  type ContainerStopResp struct {
    Message string
  }
  j := ContainerStopResp{}
  b := bytes.NewReader(body)
	json.NewDecoder(b).Decode(&j)

  //ADD VERIFICATION HERE!!!!!!!!!!!!!

  return nil
}

func DockerContainerStatus(ContainerName string) (status string, err error) {
  url := "http://d/containers/" + ContainerName + "/json"
  body, _ := DockerCallRuntimeApi("GET", url, nil)
  type DockerStatusResp struct {
    State struct {
      Status string
    }
  }
  j := DockerStatusResp{}
  b := bytes.NewReader(body)
	json.NewDecoder(b).Decode(&j)
  //ADD VERIFICATION HERE!!!!!!!!!!!!!

  return j.State.Status, nil
}
