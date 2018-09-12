package main

import (
  "fmt"
  "math/rand"
  "time"
  "net/http"

  "github.com/gorilla/mux"
	"github.com/rs/cors"
)

type MasterInfo struct {
  leader        string
  candidate     bool
  electionTimer *time.Timer
  voted         bool
}

func resetElectionTimeout(){
  randNum := rand.Intn(5) + 5
  if master.electionTimer == nil {
    master.electionTimer = time.NewTimer(time.Duration(randNum) * time.Second)
  } else {
    master.electionTimer.Reset(time.Duration(randNum) * time.Second)
  }
  fmt.Println("The new election timeout:", randNum)
}

func leaderElection() {
  fmt.Println("Becoming a candidate and running a vote.")
  master.candidate = true
  master.voted = true
  votes := 1

  for key, IP := range config.Servers {
    if IP != config.ServerIP {
      fmt.Println("Sending vote request to", key)
      if callVote(IP) {
        votes++
      }
    }
  }

  if float64(votes) / float64(len(config.Servers)) > 0.5 {
    master.candidate = false
    master.leader = config.ServerIP
    fmt.Println("This server is now the leader.")
    for ;; {
      heartbeat := time.NewTimer(1 * time.Second)
      heartbeats := 1
      for key, IP := range config.Servers {
        if IP != config.ServerIP {
          fmt.Println("Sending heartbeat to", key)
          if callHeartbeat(IP) {
            heartbeats++
          }
        }
      }
      if float64(heartbeats) / float64(len(config.Servers)) < 0.5 {
        fmt.Println("Not enough beats from followers to stay leader.")
        break
      }
      <-heartbeat.C
    }
  } else {
    master.candidate = false
    fmt.Println("Majority vote failed. Resetting election timeout")
  }
}

func callHeartbeat(server string) bool {
  req, err := http.NewRequest("GET", "http://" + server + "47433:/heartbeat", nil)
  if err != nil {
    fmt.Println(err)
    return false
  }
  client := &http.Client{}
  resp, err := client.Do(req)
  if err != nil {
    fmt.Println(err)
    return false
  }
	resp.Body.Close()

  return true
}


func callVote(server string) bool {
  req, err := http.NewRequest("GET", "http://" + server + "47433:/vote", nil)
  if err != nil {
    fmt.Println(err)
    return false
  }
  client := &http.Client{
    Timeout: time.Second,
  }
  resp, err := client.Do(req)
  if err != nil {
    fmt.Println(err)
    return false
  }

  fmt.Println("The status code is:", resp.StatusCode)
  if resp.StatusCode == http.StatusLocked {
    resp.Body.Close()
    return false
  }

	resp.Body.Close()
  return true
}

//---------------------API------------------------------------------------
func heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
  resetElectionTimeout()
}

func voteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
  if master.candidate || master.voted == true {
    w.WriteHeader(http.StatusLocked)
  } else {
    master.voted = true
    w.WriteHeader(http.StatusOK)
  }
  resetElectionTimeout()
}

func startRaftAPIServer(port string) {
  router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/heartbeat", heartbeatHandler)
  router.HandleFunc("/vote", voteHandler)

	handler := cors.Default().Handler(router)

	//Setup server config
	server := &http.Server{
		Addr:      ":" + port,
		Handler:   handler,
  }

	//Start API server
	err := server.ListenAndServe()
	panic(err)
}

func startRaft() {
  fmt.Println("Starting Raft.")
  fmt.Println("Waiting for majority of masters to come online.")
  go startRaftAPIServer("46433")
  time.Sleep(time.Second)

  //Election Timeout
  for ;; {
    resetElectionTimeout()
    master.voted = false
    <-master.electionTimer.C
    leaderElection()
  }
}


//---------------------API------------------------------------------------
