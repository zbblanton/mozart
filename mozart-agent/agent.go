package main

import(
  "net"
  "fmt"
  "sync"
  "flag"
	"log"
  "crypto/x509"
	"crypto/x509/pkix"
  "crypto/rand"
  "crypto/rsa"
  "crypto/tls"
  "net/http"
  "bufio"
  "io/ioutil"
  "io"
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

func callInsecuredServer(method string, url string, body io.Reader) (respBody []byte, err error)  {
  c := &tls.Config{
		InsecureSkipVerify: true,
	}
	tr := &http.Transport{TLSClientConfig: c}
	client := &http.Client{Transport: tr}

  req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
  resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

  reader := bufio.NewReader(resp.Body)
  respBody, _ = ioutil.ReadAll(reader)
  resp.Body.Close()

  return respBody, nil
}

func callSecuredServer(){

}

func generateCSR(privateKey *rsa.PrivateKey, Ip string) (csr []byte, err error) {
  //CSR config
  csrSubject := pkix.Name{
      Organization:  []string{"Mozart"}}
  csrConfig := &x509.CertificateRequest{
    Subject: csrSubject,
    PublicKey: privateKey,
    IPAddresses:  []net.IP{net.ParseIP(Ip)}}

  csr, err = x509.CreateCertificateRequest(rand.Reader, csrConfig, privateKey)
  if err != nil {
		return nil, err
	}

  return csr, err
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
