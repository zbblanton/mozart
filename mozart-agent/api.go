package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"log"
	"net/http"
)

//RootHandler - Handles the default route
func RootHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	defer r.Body.Close()

	j := Req{}
	json.NewDecoder(r.Body).Decode(&j)

	resp := Resp{true, ""}
	json.NewEncoder(w).Encode(resp)
}

//CreateHandler - Handles the create route
func CreateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	defer r.Body.Close()

	//Read json in and decode it
	j := CreateReq{}
	json.NewDecoder(r.Body).Decode(&j)

	fmt.Println("TEMP Received run request from master.")

	//Add to queue
	q := ControllerMsg{Action: "create", Data: j.Container}
	//containerQueue <- j.Container
	containerQueue <- q

	/*
	  if(getContainerRuntime() == "docker") {
	    container := ConvertContainerConfigToDockerContainerConfig(j.Container.Config)
	    fmt.Print(container)
	    fmt.Println(" ")
	    id, _ := DockerCreateContainer(j.Container.Name, container)
	    fmt.Print(id)
	    fmt.Println(" ")
	    DockerStartContainer(id)
	  }
	*/

	p := Resp{true, ""}
	json.NewEncoder(w).Encode(p)
}

//StopHandler - Handles the stop route
func StopHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	r.Body.Close()

	vars := mux.Vars(r)
	containerName := vars["container"]

	if containerName != "" {
		fmt.Println("Stopping container: ", containerName)
		//DockerStopContainer(containerName)

		//Add to queue
		q := ControllerMsg{Action: "stop", Data: containerName}
		//containerQueue <- containerName
		containerQueue <- q

		p := Resp{true, ""}
		json.NewEncoder(w).Encode(p)
	} else {
		p := Resp{false, "Must provide a container name or ID."}
		json.NewEncoder(w).Encode(p)
	}
}

//HealthHandler - Handles the health checking route
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	r.Body.Close()

	type healthCheck struct {
		Health  string
		Success bool
		Error   string
	}

	p := healthCheck{"ok", true, ""}
	json.NewEncoder(w).Encode(p)
}

//JoinHandler - Handles the join route
func JoinHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	defer r.Body.Close()

	type healthCheck struct {
		Health  string
		Success bool
		Error   string
	}

	p := healthCheck{"ok", true, ""}
	json.NewEncoder(w).Encode(p)
}

func startAgentAPI(port string) {
	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/", RootHandler)
	router.HandleFunc("/create", CreateHandler)
	router.HandleFunc("/list", RootHandler)
	router.HandleFunc("/stop/{container}", StopHandler)
	router.HandleFunc("/status/{container}", RootHandler)
	router.HandleFunc("/inspect/{container}", RootHandler)
	router.HandleFunc("/health", HealthHandler)

	handler := cors.Default().Handler(router)
	//err := http.ListenAndServe(":8080", handler)

	//Setup TLS config
	rootCaPool := x509.NewCertPool()
	if ok := rootCaPool.AppendCertsFromPEM([]byte(caTLSCert)); !ok {
		panic("Cannot parse root CA.")
	}
	//load signed keypair
	signedKeyPair, err := tls.X509KeyPair(agentTLSCert, agentTLSKey)
	if err != nil {
		panic(err)
	}
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{signedKeyPair},
		RootCAs:      rootCaPool,
		ClientCAs:    rootCaPool,
		ClientAuth:   tls.RequireAndVerifyClientCert}

	//Setup server config
	/*server := &http.Server{
	  Addr: "10.0.0.28" + ":" + "8080",
	  Handler: handler,
	  TLSConfig: tlsCfg}*/
	//Changed to listen on all interfaces.
	server := &http.Server{
		Addr:      ":" + port,
		Handler:   handler,
		TLSConfig: tlsCfg}

	//Start API server
	err = server.ListenAndServeTLS("", "")
	log.Fatal(err)
}
