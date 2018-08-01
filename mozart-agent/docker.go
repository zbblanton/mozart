package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/docker/go-connections/nat"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"errors"
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
func DockerCreateContainer(ContainerName string, c ContainerConfig) (id string, err error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.WithVersion("1.33"))
	if err != nil {
		return "", err
	}

	labels := make(map[string]string)
	labels["mozart"] = "true"
	//exposedPorts := make(map[string]struct{})
	exposedPorts := make(nat.PortSet)
	for _, port := range c.ExposedPorts {
		newPort, _ := nat.NewPort("tcp", port.ContainerPort)
		exposedPorts[newPort] = struct{}{}
	}
	containerConfig := &container.Config{
		Image: c.Image,
		Labels: labels,
		Env: c.Env,
		ExposedPorts: exposedPorts,
	}

	portBindings := make(nat.PortMap)
	for _, port := range c.ExposedPorts {
		p := DockerContainerHostConfigPortBindings{}
		p.HostIP = port.HostIP
		p.HostPort = port.HostPort
		newPort, _ := nat.NewPort("tcp", port.HostPort)
		portBinding := nat.PortBinding{HostIP: port.HostIP, HostPort: port.HostPort}
		portBindings[newPort] = []nat.PortBinding{portBinding}
	}
	mounts := []mount.Mount{}
	for _, m := range c.Mounts{
		var mountType mount.Type
		switch m.Type {
		case "bind":
			mountType = mount.TypeBind
		default:
			return "", errors.New("Mount type not supported.")
		}
		mounts = append(mounts, mount.Mount{Type: mountType, Source: m.Source, Target: m.Target, ReadOnly: m.ReadOnly})
	}
	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		Mounts: mounts,
		AutoRemove: true,
		Privileged: c.Privileged,
	}

	resp, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, ContainerName)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

//DockerListByID - List all the mozart tagged containers by ID
func DockerListByID() (containerList []string, err error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.WithVersion("1.33"))
	if err != nil {
		return []string{}, err
	}
	labelArg := filters.Arg("label", "mozart")
	args := filters.NewArgs(labelArg)
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{Filters: args})
	if err != nil {
		return []string{}, err
	}

	for _, container := range containers {
		containerList = append(containerList, container.ID)
	}

	return containerList, nil
}

//DockerListByName - List all the mozart tagged containers by Name
func DockerListByName() (containerList []string, err error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.WithVersion("1.33"))
	if err != nil {
		return []string{}, err
	}
	labelArg := filters.Arg("label", "mozart")
	args := filters.NewArgs(labelArg)
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{Filters: args})
	if err != nil {
		return []string{}, err
	}

	for _, container := range containers {
		name := container.Names[0][1:]
		fmt.Println(name)
		containerList = append(containerList, name)
	}

	return containerList, nil
}

//DockerGetID - Get the id of a running docker container
func DockerGetID(ContainerName string) (ID string, err error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.WithVersion("1.33"))
	if err != nil {
		return "", err
	}
	labelArg := filters.Arg("name", ContainerName)
	args := filters.NewArgs(labelArg)
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{Filters: args})
	if err != nil || len(containers) == 0 {
		return "", err
	}

	return containers[0].ID, nil
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
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.WithVersion("1.33"))
	if err != nil {
		return err
	}
	if err = cli.ContainerStart(ctx, ContainerID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	return nil
}

//DockerStopContainer - Stops a docker container
func DockerStopContainer(ContainerID string) error {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.WithVersion("1.33"))
	if err != nil {
		return err
	}
	if err = cli.ContainerStop(ctx, ContainerID, nil); err != nil {
		return err
	}

	return nil
}

//DockerContainerStatus - Gets the status of a docker container
func DockerContainerStatus(ContainerName string) (status string, err error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.WithVersion("1.33"))
	if err != nil {
		return "", err
	}
	labelArg := filters.Arg("name", ContainerName)
	args := filters.NewArgs(labelArg)
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{Filters: args})
	if err != nil || len(containers) == 0 {
		return "", err
	}

	return containers[0].State, nil
}
