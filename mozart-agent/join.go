package main

import(
  "os"
  "bytes"
  "fmt"
	"encoding/json"
  "crypto/rand"
	"encoding/base64"
  "crypto/rsa"
	"crypto/x509"
  "crypto/sha256"
  "encoding/pem"
)

//Step 1: Generate a Key and CSR
//Step 2: Send join key and CSR and receive CA
//Step 3: Verify CA hash matches our hash and save Cert
//Step 4: Send IP, name, join key, agent key. Receive server key
func joinAgent(serverIP string, agentIP string, joinKey string, agentCaHash string) string{
  //Step 1
  fmt.Println("Starting Join Process...")
  fmt.Println("Generating Private Key...")
  privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
  agentTLSKey = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
  fmt.Println("Generating CSR...")
  csr, err := generateCSR(privateKey, agentIP)
  if err != nil {
		panic(err)
	}

  //Step 2
  fmt.Println("Encoding CSR...")
  csrString := base64.URLEncoding.EncodeToString(csr)

  type NodeInitialJoinReq struct {
    AgentIP string
    JoinKey string
    Csr string
  }

  j := NodeInitialJoinReq{AgentIP: agentIP, JoinKey: joinKey, Csr: csrString}
  b := new(bytes.Buffer)
  json.NewEncoder(b).Encode(j)
  fmt.Println("Sending initial join request...")

  resp, err := callInsecuredServer("POST", "https://" + serverIP + ":48433/nodes/initialjoin", b)
  if err != nil {
		panic(err)
	}

  type NodeInitialJoinResp struct {
    CaCert string
    ClientCert string
    Success bool `json:"success"`
    Error string `json:"error"`
  }

  respj := NodeInitialJoinResp{}
  err = json.Unmarshal(resp, &respj)
	if err != nil {
		fmt.Println("error:", err)
	}

  fmt.Println("Received response: ", respj)

  //Step 3
  if !respj.Success {
    panic(respj.Error)
  }

  //Decode CA
  fmt.Println("Decoding server CA...")
  ca, err := base64.URLEncoding.DecodeString(respj.CaCert)
  if err != nil {
    panic(err)
  }

  //Save the CA
  fmt.Println("Saving CA...")
  caTLSCert = ca

  //Decode agent CA hash, Compute hash, and compare
  agentCaHashDecoded, err := base64.URLEncoding.DecodeString(agentCaHash)
  if err != nil {
    panic(err)
  }
  fmt.Println("Comparing CA hash to our hash to validate server...")

  caHash := sha256.Sum256(ca)
  sliceCaHash := caHash[:] //Quickfix to convert [32]byte to []byte so that we can compare
  if(!bytes.Equal(sliceCaHash, agentCaHashDecoded)){
    panic("Hashes are not equal! Cannot trust server!")
  }

  //Decode and save cert
  cert, err := base64.URLEncoding.DecodeString(respj.ClientCert)
  if err != nil {
    panic(err)
  }
  fmt.Println("Saving agent cert")
  agentTLSCert = cert

  //Step 4 (NEED TO ADD CA TO POST!!!)
  fmt.Println("The join key is: ", joinKey)
  //Generating key taken from http://blog.questionable.services/article/generating-secure-random-numbers-crypto-rand/
  //Generate random key
  randKey := make([]byte, 128)
  _, err = rand.Read(randKey)
  if err != nil {
    fmt.Println("Error generating a new worker key, we are going to exit here due to possible system errors.")
    os.Exit(1)
  }
  agentAuthKey := base64.URLEncoding.EncodeToString(randKey)

  fmt.Println("The agent auth key is: ", agentAuthKey)

  type NodeJoinReq struct {
    JoinKey string
    AgentKey string
    Type string
    AgentIP string
    AgentPort string
  }

  joinReq := NodeJoinReq{JoinKey: joinKey, AgentKey: agentAuthKey, Type: "worker", AgentIP: agentIP, AgentPort: "8080"}
  b2 := new(bytes.Buffer)
  json.NewEncoder(b2).Encode(joinReq)

  url := "https://" + serverIP + ":47433/nodes/join"

  fmt.Println("Sending secured join request...")

  secureResp, err := callSecuredServer(agentTLSCert, agentTLSKey, caTLSCert, "POST", url, b2)
  if err != nil {
		panic(err)
	}

  type NodeJoinResp struct {
    ServerKey string
    Containers map[string]Container
    Success bool `json:"success"`
    Error string `json:"error"`
  }
  joinResp := NodeJoinResp{}

  err = json.Unmarshal(secureResp, &joinResp)
	if err != nil {
		fmt.Println("error:", err)
	}
  if !joinResp.Success {
    fmt.Println("Error:", joinResp.Error)
    os.Exit(1)
  }
  fmt.Println("The secured join request response: ", joinResp)

  if len(joinResp.Containers) == 0 {
    stopAllMozartContainers()
  } else {
    //Get IDs for mozart Containers
    var keepRunningList []string
    for _, container := range joinResp.Containers {
      id, _ := DockerGetID(container.Name)
      keepRunningList = append(keepRunningList, id)
    }

    currentRunningList, _ := DockerList()
    for _, containerID := range currentRunningList {
      found := false
      for _, keepRunningItem := range keepRunningList {
        if containerID == keepRunningItem {
          found = true
          break
        }
      }
      if !found {
        DockerStopContainer(containerID)
      }
    }
  }

  //Save existing containers if they exist
  containers.mux.Lock()
  containers.Containers = joinResp.Containers
  fmt.Println("Containers that should be running on this worker:")
  fmt.Println(containers.Containers)
  //for _, container := range containers.Containers {
  //  fmt.Println(container)
  //}
  containers.mux.Unlock()

  return joinResp.ServerKey
}
