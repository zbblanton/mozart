package main

import(
  "net"
  "fmt"
  "sync"
  "flag"
	"log"
)

type ExposedPort struct {
	ContainerPort string
	HostPort string
	HostIp string
}

type Mount struct {
	Target string
	Source string
	Type string
	ReadOnly bool
}

type ContainerConfig struct {
	Name string
	Image string
	ExposedPorts []ExposedPort
	Mounts []Mount
	Env []string
	AutoRemove bool
	Privileged bool
}
/*
type DockerContainerHostConfigMounts struct {
  Target string
	Source string
	Type string
	ReadOnly bool
}
*/
type DockerContainerHostConfigPortBindings struct {
  HostIp string
  HostPort string
}

type DockerContainerHostConfig struct {
  PortBindings map[string][]DockerContainerHostConfigPortBindings
  Mounts []Mount
  AutoRemove bool
  Privileged bool
}

type DockerContainerConfig struct {
  Image string
  Env []string
  Labels map[string]string
  ExposedPorts map[string]struct{}
  HostConfig DockerContainerHostConfig
}

type CreateReq struct {
  Key string
  Container ContainerConfig
}

type StopReq struct {
  Key string
  ContainerName string
}

type CreateResp struct {
  Success bool `json:"success"`
  Error string `json:"error"`
}

type Req struct {
  Key string `json:"key"`
  Command string `json:"command"`
}

type Resp struct {
  Success bool `json:"success"`
  Error string `json:"error"`
}

type Config struct {
  ServerKey string
  mux sync.Mutex
}

func (c *Config) getServerKey() string {
  c.mux.Lock()
  serverKey := c.ServerKey
  c.mux.Unlock()
  return serverKey
}

func fakeDial(proto, addr string) (conn net.Conn, err error) {
  return net.Dial("unix", "/var/run/docker.sock")
}

func getContainerRuntime() string {
  return "docker"
}

var config = Config{ServerKey: ""}
var agentTlsKey = []byte{}
var agentTlsCert = []byte{}
var caTLSCert = []byte{}

func main() {
  agentPtr := flag.String("agent", "", "Hostname or IP to use for this agent. (Required)")
  serverPtr := flag.String("server", "", "Hostname or IP of the mozart server. (Required)")
  keyPtr := flag.String("key", "", "Mozart join key to join the cluster. (Required)")
  caHashPtr := flag.String("ca-hash", "", "Mozart CA hash to verify server CA. (Required)")
  flag.Parse()
  if(*agentPtr == ""){
    log.Fatal("Must provide this nodes Hostname or IP.")
  }
  if(*serverPtr == ""){
    log.Fatal("Must provide a server.")
  }
  if(*keyPtr == ""){
    log.Fatal("Must provide a join key to join the cluster.")
  }
  if(*caHashPtr == ""){
    log.Fatal("Must provide a CA hash to verify the cluster CA.")
  }
  fmt.Println("Joining agent to " + *serverPtr + "...")
  config.ServerKey = joinAgent(*serverPtr, *agentPtr, *keyPtr, *caHashPtr)
  if(config.ServerKey == ""){
    log.Fatal("Something went wrong when trying to join the agent.")
  }
  fmt.Println("Agent successfully joined the cluster.")
  //go MonitorContainers()
	startAgentApi()
}
