package main

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func containersCreateVerification(c ContainerConfig) bool {
	return true
}

//RootHandler - Handles any unrouted requests.
func RootHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hi there :)\n"))
}

//NodeInitialJoinHandler - Node Initial Join
func NodeInitialJoinHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	defer r.Body.Close()

	j := NodeInitialJoinReq{}
	json.NewDecoder(r.Body).Decode(&j)

	//Verify key
	if j.JoinKey != config.AgentJoinKey {
		fmt.Println(config.AgentJoinKey)
		fmt.Println(j.JoinKey)
		resp := Resp{Success: false, Error: "Invalid join key"}
		json.NewEncoder(w).Encode(resp)
		return
	}

	//Decode the CSR from base64
	csr, err := base64.URLEncoding.DecodeString(j.Csr)
	if err != nil {
		resp := Resp{Success: false, Error: "Error during initial join."}
		json.NewEncoder(w).Encode(resp)
		return
	}

	//Sign the CSR
	signedCert, err := signCSR(config.CaCert, config.CaKey, csr, j.AgentIP)
	if err != nil {
		resp := Resp{Success: false, Error: "Error during initial join."}
		json.NewEncoder(w).Encode(resp)
		return
	}

	//Prepare signed cert to be sent to agent
	signedCertString := base64.URLEncoding.EncodeToString(signedCert)

	//Prepare CA to be sent to agent
	ca, err := ioutil.ReadFile(config.CaCert)
	if err != nil {
		resp := Resp{Success: false, Error: "Error during initial join."}
		json.NewEncoder(w).Encode(resp)
		return
	}
	caString := base64.URLEncoding.EncodeToString(ca)

	resp := NodeInitialJoinResp{caString, signedCertString, true, ""}
	json.NewEncoder(w).Encode(resp)
}

//NodeJoinHandler - Node join handler
func NodeJoinHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	defer r.Body.Close()

	j := NodeJoinReq{}
	json.NewDecoder(r.Body).Decode(&j)

	//ADD VERIFICATION!!!!

	//Verify key
	if j.JoinKey != config.AgentJoinKey {
		fmt.Println(config.AgentJoinKey)
		fmt.Println(j.JoinKey)
		resp := Resp{Success: false, Error: "Invalid join key"}
		json.NewEncoder(w).Encode(resp)
		return
	}

	//Check if worker exist and if it has an active or maintenance status
	//if worker, ok := workers["mozart/workers/" + j.AgentIP]; ok {
	var worker Worker
	workerBytes, _ := ds.Get("mozart/workers/" + j.AgentIP)
	if workerBytes != nil {
		err := json.Unmarshal(workerBytes, &worker)
		if err != nil {
			eventError(err)
			resp := Resp{Success: false, Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}

		if worker.Status == "active" || worker.Status == "connected" || worker.Status == "maintenance" {
			resp := Resp{Success: false, Error: "Host already exist and has an active or maintenance status. (This is okay if host is rejoining, just retry until it reconnects!)"}
			json.NewEncoder(w).Encode(resp)
			return
		}
	}

	//Generating key taken from http://blog.questionable.services/article/generating-secure-random-numbers-crypto-rand/
	//Generate random key
	randKey := make([]byte, 128)
	_, err := rand.Read(randKey)
	if err != nil {
		fmt.Println("Error generating a new worker key, we are going to exit here due to possible system errors.")
		os.Exit(1)
	}
	serverKey := base64.URLEncoding.EncodeToString(randKey)
	//Save key to config
	var newWorker Worker
	if workerBytes == nil {
		newWorker = Worker{AgentIP: j.AgentIP, AgentPort: "49433", ServerKey: serverKey, AgentKey: j.AgentKey, Containers: make(map[string]string), Status: "active"}
	} else {
		newWorker = Worker{AgentIP: j.AgentIP, AgentPort: "49433", ServerKey: serverKey, AgentKey: j.AgentKey, Containers: worker.Containers, Status: "active"}
	}

	b, err := json.Marshal(newWorker)
	if err != nil {
		eventError(err)
		resp := Resp{Success: false, Error: err.Error()}
		json.NewEncoder(w).Encode(resp)
		return
	}
	ds.Put("mozart/workers/"+j.AgentIP, b)

	//Create containers map
	workerContainers := make(map[string]Container)

	//Get each container and add to map
	for _, containerName := range worker.Containers {
		var container Container
		c, _ := ds.Get("mozart/containers/" + containerName)
		err = json.Unmarshal(c, &container)
		if err != nil {
			eventError(err)
			resp := Resp{Success: false, Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		workerContainers[containerName] = container
	}

	resp := NodeJoinResp{ServerKey: serverKey, Containers: workerContainers, Success: true, Error: ""}
	json.NewEncoder(w).Encode(resp)
}

//ContainersCreateHandler - Create container
func ContainersCreateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	defer r.Body.Close()

	j := ContainerConfig{}
	json.NewDecoder(r.Body).Decode(&j)
	if containersCreateVerification(j) {
		fmt.Println("Received a run request for config: ", j, "adding to queue.")
		containerQueue <- j
		resp := Resp{true, ""}
		json.NewEncoder(w).Encode(resp)
	} else {
		resp := Resp{false, "Invalid data"} //Add better error.
		json.NewEncoder(w).Encode(resp)
	}
}

//ContainersStopHandler - stop container
func ContainersStopHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	defer r.Body.Close()

	vars := mux.Vars(r)
	containerName := vars["container"]
	if containerName == "" {
		resp := Resp{false, "Must provide a container name."}
		json.NewEncoder(w).Encode(resp)
		return
	}

	//Check if container exist
	//containers.mux.Lock()
	if ok, _ := ds.ifExist("mozart/containers/" + containerName); !ok {
		resp := Resp{false, "Cannot find container"}
		json.NewEncoder(w).Encode(resp)
	} else {
		resp := Resp{true, ""}
		json.NewEncoder(w).Encode(resp)
		//Add to queue
		containerQueue <- containerName
	}
}

//ContainersStateUpdateHandler - Update container state
func ContainersStateUpdateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	defer r.Body.Close()

	type StateUpdateReq struct {
		Key           string
		ContainerName string
		State         string
	}

	j := StateUpdateReq{}
	json.NewDecoder(r.Body).Decode(&j)

	//TODO: Verify Worker Key here, the container must live on this host.
	//containers.mux.Lock()
	fmt.Print(j)
	var container Container
	c, _ := ds.Get("mozart/containers/" + j.ContainerName)
	err := json.Unmarshal(c, &container)
	if err != nil {
		eventError(err)
		resp := Resp{Success: false, Error: err.Error()}
		json.NewEncoder(w).Encode(resp)
		return
	}
	if j.State == "stopped" && container.DesiredState == "stopped" {
		ds.Del("mozart/containers/" + container.Name)
		//Update worker container run list
		var worker Worker
		workerBytes, _ := ds.Get("mozart/workers/" + container.Worker)
		err = json.Unmarshal(workerBytes, &worker)
		if err != nil {
			eventError(err)
			resp := Resp{Success: false, Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		delete(worker.Containers, container.Name)
		workerToBytes, err := json.Marshal(worker)
		if err != nil {
			eventError(err)
			resp := Resp{Success: false, Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		ds.Put("mozart/workers/"+container.Worker, workerToBytes)
	} else {
		container.State = j.State
		fmt.Print(container)
		b, err := json.Marshal(container)
		if err != nil {
			eventError(err)
			resp := Resp{Success: false, Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		ds.Put("mozart/containers/"+container.Name, b)
	}

	resp := Resp{true, ""}
	json.NewEncoder(w).Encode(resp)
}

//ContainersListHandler - List all containers
func ContainersListHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	defer r.Body.Close()

	containers := make(map[string]Container)

	//Get containers
	dataBytes, _ := ds.GetByPrefix("mozart/containers")
	for k, v := range dataBytes {
		var data Container
		err := json.Unmarshal(v, &data)
		if err != nil {
			eventError(err)
			resp := Resp{Success: false, Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		containers[k] = data
	}

	resp := ContainerListResp{containers, true, ""}
	json.NewEncoder(w).Encode(resp)
}

//CheckAccountAuth - Middleware to handle auth
func CheckAccountAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//Get Auth information from headers
		headerAccount := r.Header.Get("Account")
		headerAccessKey := r.Header.Get("Access-Key")
		headerSecretKey := r.Header.Get("Secret-Key")

		//Check if Form values have been provided
		if headerAccount == "" || headerAccessKey == "" || headerSecretKey == "" {
			resp := Resp{false, "Must provide an account, access key, and secret key."}
			json.NewEncoder(w).Encode(resp)
			return
		}

		//Get account from datastore
		var account Account
		accountBytes, err := ds.Get("mozart/accounts/" + headerAccount)
		if accountBytes == nil {
			resp := Resp{false, "Invalid Auth. Not accounts found. (This warning is temp)"}
			json.NewEncoder(w).Encode(resp)
			return
		}
		if err != nil {
			resp := Resp{false, "Error trying to auth."}
			json.NewEncoder(w).Encode(resp)
			return
		}
		err = json.Unmarshal(accountBytes, &account)
		if err != nil {
			resp := Resp{false, "Could not Unmarshal. Invalid Auth."}
			json.NewEncoder(w).Encode(resp)
			return
		}

		//Verify auth
		if headerAccessKey != account.AccessKey || headerSecretKey != account.SecretKey {
			resp := Resp{false, "Invalid Auth."}
			json.NewEncoder(w).Encode(resp)
			return
		}

		handler(w, r)
	}
}

//AccountsCreateHandler - Create an account
func AccountsCreateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	defer r.Body.Close()

	j := Account{}
	json.NewDecoder(r.Body).Decode(&j)

	if j.Name == "" {
		resp := Resp{false, "Must provide an account name."}
		json.NewEncoder(w).Encode(resp)
		return
	}

	//Check if account exist
	if ok, _ := ds.ifExist("mozart/accounts/" + j.Name); ok {
		resp := Resp{false, "Account already exists!"}
		json.NewEncoder(w).Encode(resp)
		return
	}

	//Generate Access Key
	randKey := make([]byte, 16)
	_, err := rand.Read(randKey)
	if err != nil {
		fmt.Println("Error generating a key, we are going to exit here due to possible system errors.")
		os.Exit(1)
	}
	j.AccessKey = base64.URLEncoding.EncodeToString(randKey)

	//Generate Secret Key
	randKey = make([]byte, 64)
	_, err = rand.Read(randKey)
	if err != nil {
		fmt.Println("Error generating a key, we are going to exit here due to possible system errors.")
		os.Exit(1)
	}
	j.SecretKey = base64.URLEncoding.EncodeToString(randKey)

	//Save account
	accountBytes, err := json.Marshal(j)
	if err != nil {
		eventError(err)
		resp := Resp{Success: false, Error: err.Error()}
		json.NewEncoder(w).Encode(resp)
		return
	}
	ds.Put("mozart/accounts/"+j.Name, accountBytes)

	type AccountCreateResp struct {
		Account Account
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}

	//Send keys back.
	resp := AccountCreateResp{j, true, ""}
	json.NewEncoder(w).Encode(resp)
}

//AccountsListHandler - List all accounts
func AccountsListHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	defer r.Body.Close()

	accounts := make(map[string]Account)

	//Get accounts
	dataBytes, _ := ds.GetByPrefix("mozart/accounts")
	for k, v := range dataBytes {
		var data Account
		err := json.Unmarshal(v, &data)
		if err != nil {
			eventError(err)
			resp := Resp{Success: false, Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		data.AccessKey = ""
		data.SecretKey = ""
		data.Password = ""
		accounts[k] = data
	}

	resp := AccountsListResp{accounts, true, ""}
	json.NewEncoder(w).Encode(resp)
}

//WorkersListHandler - List all workers
func WorkersListHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	defer r.Body.Close()

	workers := make(map[string]Worker)

	//Get accounts
	dataBytes, _ := ds.GetByPrefix("mozart/workers")
	for k, v := range dataBytes {
		var data Worker
		err := json.Unmarshal(v, &data)
		if err != nil {
			eventError(err)
			resp := Resp{Success: false, Error: err.Error()}
			json.NewEncoder(w).Encode(resp)
			return
		}
		data.ServerKey = ""
		data.AgentKey = ""
		workers[k] = data
	}

	resp := NodeListResp{workers, true, ""}
	json.NewEncoder(w).Encode(resp)
}

//ClusterConfigHandler - Send server config data
func ClusterConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	defer r.Body.Close()

	type ClusterConfigResp struct {
		Server  string
		Ca      string
		CaHash  string
		JoinKey string
		Success bool
		Error   string
	}

	caHashBytes := sha256.Sum256(caTLSCert)
	caHashSlice := caHashBytes[:]
	caHash := base64.URLEncoding.EncodeToString(caHashSlice)
	ca := string(caTLSCert)

	resp := ClusterConfigResp{config.ServerIP, ca, caHash, config.AgentJoinKey, true, ""}
	json.NewEncoder(w).Encode(resp)
}

func startAccountAndJoinServer(serverIP string, joinPort string, caCert string, serverCert string, serverKey string) {
	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/nodes/initialjoin", NodeInitialJoinHandler)

	router.HandleFunc("/containers/create", CheckAccountAuth(ContainersCreateHandler))
	router.HandleFunc("/containers/stop/{container}", CheckAccountAuth(ContainersStopHandler))
	router.HandleFunc("/containers/list", CheckAccountAuth(ContainersListHandler))

	router.HandleFunc("/accounts/create", CheckAccountAuth(AccountsCreateHandler))
	router.HandleFunc("/accounts/remove", CheckAccountAuth(RootHandler))
	router.HandleFunc("/accounts/list", CheckAccountAuth(AccountsListHandler))

	router.HandleFunc("/workers/list/", CheckAccountAuth(WorkersListHandler))

	router.HandleFunc("/cluster/config", CheckAccountAuth(ClusterConfigHandler))

	handler := cors.Default().Handler(router)

	//Setup server config
	server := &http.Server{
		//Addr:    serverIP + ":" + joinPort,
		Addr:    ":" + joinPort,
		Handler: handler}

	//Start Join server
	err := server.ListenAndServeTLS(serverCert, serverKey)
	log.Fatal(err)
}

func startAPIServer(serverIP string, serverPort string, caCert string, serverCert string, serverKey string) {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", RootHandler)

	router.HandleFunc("/containers/create", ContainersCreateHandler)
	router.HandleFunc("/containers/stop/{container}", ContainersStopHandler)
	router.HandleFunc("/containers/list", ContainersListHandler)
	//router.HandleFunc("/containers/list/{worker}", ContainersListWorkersHandler)
	router.HandleFunc("/containers/{container}/state/update", ContainersStateUpdateHandler)
	router.HandleFunc("/containers/status/{container}", RootHandler)
	router.HandleFunc("/containers/inspect/{container}", RootHandler)

	router.HandleFunc("/workers/list/", WorkersListHandler)

	//router.HandleFunc("/nodes/list", NodeListHandler)
	router.HandleFunc("/nodes/list/{type}", RootHandler)
	router.HandleFunc("/nodes/join", NodeJoinHandler)

	router.HandleFunc("/service/create", RootHandler)
	router.HandleFunc("/service/list", RootHandler)
	router.HandleFunc("/service/inspect", RootHandler)

	router.HandleFunc("/accounts/create", AccountsCreateHandler)
	router.HandleFunc("/accounts/remove", RootHandler)
	router.HandleFunc("/accounts/list", AccountsListHandler)

	router.HandleFunc("/cluster/config", ClusterConfigHandler)

	handler := cors.Default().Handler(router)

	//Setup TLS config
	rootCa, err := ioutil.ReadFile(caCert)
	if err != nil {
		panic("cant open file")
	}
	rootCaPool := x509.NewCertPool()
	if ok := rootCaPool.AppendCertsFromPEM([]byte(rootCa)); !ok {
		panic("Cannot parse root CA.")
	}
	tlsCfg := &tls.Config{
		RootCAs:    rootCaPool,
		ClientCAs:  rootCaPool,
		ClientAuth: tls.RequireAndVerifyClientCert}

	//Setup server config
	server := &http.Server{
		//Addr:      serverIP + ":" + serverPort,
		Addr:      ":" + serverPort,
		Handler:   handler,
		TLSConfig: tlsCfg}

	//Start API server
	err = server.ListenAndServeTLS(serverCert, serverKey)
	log.Fatal(err)
}
