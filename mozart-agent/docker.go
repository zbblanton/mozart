package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"net"
	"net/http"
)

//ConvertContainerConfigToDockerContainerConfig - Converts a config to a docker compatible config
func ConvertContainerConfigToDockerContainerConfig(c ContainerConfig) DockerContainerConfig {
	d := DockerContainerConfig{}
	d.Image = c.Image
	d.Env = c.Env
	d.Labels = make(map[string]string)
	d.Labels["mozart"] = "true"
	d.ExposedPorts = make(map[string]struct{})
	for _, port := range c.ExposedPorts {
		d.ExposedPorts[port.ContainerPort+"/tcp"] = struct{}{}
	}
	d.HostConfig.PortBindings = make(map[string][]DockerContainerHostConfigPortBindings)
	for _, port := range c.ExposedPorts {
		p := DockerContainerHostConfigPortBindings{}
		p.HostIP = port.HostIP
		p.HostPort = port.HostPort
		d.HostConfig.PortBindings[port.ContainerPort+"/tcp"] = []DockerContainerHostConfigPortBindings{p}
	}
	d.HostConfig.Mounts = c.Mounts
	d.HostConfig.AutoRemove = c.AutoRemove
	d.HostConfig.Privileged = c.Privileged

	return d
}

func fakeDial(proto, addr string) (conn net.Conn, err error) {
	return net.Dial("unix", "/var/run/docker.sock")
}

//DockerCallRuntimeAPI - Calls the Docker Runtime API
func DockerCallRuntimeAPI(method string, url string, body io.Reader) (respBody []byte, err error) {
	tr := &http.Transport{
		Dial: fakeDial,
	}

	client := &http.Client{Transport: tr}
	b := new(bytes.Buffer)          //I dont think we need this line
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

//DockerCreateContainer - Creates a docker container
func DockerCreateContainer(ContainerName string, Container DockerContainerConfig) (id string, err error) {
	buff := new(bytes.Buffer)
	json.NewEncoder(buff).Encode(Container)
	url := "http://d/containers/create"
	if ContainerName != "" {
		url = url + "?name=" + ContainerName
	}

	body, _ := DockerCallRuntimeAPI("POST", url, buff)

	type ContainerCreateResp struct {
		ID       string
		Warnings string
		Message  string
	}
	j := ContainerCreateResp{}
	b := bytes.NewReader(body)
	json.NewDecoder(b).Decode(&j)

	fmt.Println("Response from Docker Runtime API:", j)

	//ADD VERIFICATION HERE!!!!!!!!!!!!!

	return j.ID, nil
}

//DockerList - List all the mozart tagged containers
func DockerList() (containerList []string, err error) {
	url := "http://d/containers/" + "json?filters=%7B%22label%22%3A%5B%22mozart%22%5D%7D"
	//url := "http://d/containers/" + "json"
	body, err := DockerCallRuntimeAPI("GET", url, nil)
	if err != nil {
		fmt.Println("Error trying to get docker list:", err)
	}
	/*type DockerListResp struct {
	  List []struct {
	    ID string
	  }
	}*/
	type DockerListItem struct {
		ID string
	}
	j := []DockerListItem{}
	b := bytes.NewReader(body)
	json.NewDecoder(b).Decode(&j)
	//fmt.Println(j)
	//ADD VERIFICATION HERE!!!!!!!!!!!!!

	for _, container := range j {
		//fmt.Println("Container:", container)
		containerList = append(containerList, container.ID)
	}

	return containerList, nil
}

//DockerGetID - Get the id of a running docker container
func DockerGetID(ContainerName string) (ID string, err error) {
	url := "http://d/containers/" + ContainerName + "/json"
	body, _ := DockerCallRuntimeAPI("GET", url, nil)
	type DockerStatusResp struct {
		ID string
	}
	j := DockerStatusResp{}
	b := bytes.NewReader(body)
	json.NewDecoder(b).Decode(&j)
	//ADD VERIFICATION HERE!!!!!!!!!!!!!

	return j.ID, nil
}

//DockerPullImage - Pulls a docker image down to the host.
func DockerPullImage(imageName string) error {
	/*encodedImageName := url.QueryEscape(imageName)
	  url := "http://d/images/create?fromImage=" + encodedImageName
	  _, err := DockerCallRuntimeAPI("POST", url, bytes.NewBuffer([]byte(`{ }`)))
	  //fmt.Println(string(body[:]))
	  if err != nil {
	    //fmt.Println("Error trying to pull image:", string(body[:]))
	  }
	  return err*/

	ctx := context.Background()
	//cli, err := client.NewEnvClient()
	cli, err := client.NewClientWithOpts(client.WithVersion("1.33"))
	if err != nil {
		panic(err)
	}

	out, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	//We use bufio and readALL to force a wait on our image pull
	fmt.Println("Pulling image if needed...")
	reader := bufio.NewReader(out)
	ioutil.ReadAll(reader)
	out.Close()
	fmt.Println("Image ready.")
	return err
}

//DockerStartContainer - Starts a docker container
func DockerStartContainer(ContainerID string) error {
	url := "http://d/containers/" + ContainerID + "/start"
	body, _ := DockerCallRuntimeAPI("POST", url, bytes.NewBuffer([]byte(`{	}`)))
	type ContainerStartResp struct {
		Message string
	}
	j := ContainerStartResp{}
	b := bytes.NewReader(body)
	json.NewDecoder(b).Decode(&j)

	//ADD VERIFICATION HERE!!!!!!!!!!!!!

	return nil
}

//DockerStopContainer - Stops a docker container
func DockerStopContainer(ContainerID string) error {
	url := "http://d/containers/" + ContainerID + "/stop"
	body, _ := DockerCallRuntimeAPI("POST", url, bytes.NewBuffer([]byte(`{	}`)))
	type ContainerStopResp struct {
		Message string
	}
	j := ContainerStopResp{}
	b := bytes.NewReader(body)
	json.NewDecoder(b).Decode(&j)

	//ADD VERIFICATION HERE!!!!!!!!!!!!!

	return nil
}

//DockerContainerStatus - Gets the status of a docker container
func DockerContainerStatus(ContainerName string) (status string, err error) {
	url := "http://d/containers/" + ContainerName + "/json"
	body, _ := DockerCallRuntimeAPI("GET", url, nil)
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
