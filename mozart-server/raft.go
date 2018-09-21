package main

import (
  "bytes"
  "fmt"
  "math/rand"
  "time"
  "net/http"
  "encoding/json"

  "github.com/gorilla/mux"
	"github.com/rs/cors"
)

type MasterInfo struct {
  leader        string
  candidate     bool
  electionTimer *time.Timer
  votedFor      string
  currentServer string
}

type raftReq struct {
  Server  string
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
  master.votedFor = master.currentServer
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
    master.leader = master.currentServer
    fmt.Println("This server is now the leader.")
    for ;; {
      heartbeat := time.NewTimer(1 * time.Second)
      heartbeats := 1
      for key, IP := range config.Servers {
        if IP != master.currentServer {
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
	j := raftReq{Server: master.currentServer}
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(j)
  req, err := http.NewRequest("GET", "http://" + server + ":46433/heartbeat", b)
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
	j := raftReq{Server: master.currentServer}
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(j)
  req, err := http.NewRequest("GET", "http://" + server + ":46433/vote", b)
  if err != nil {
    fmt.Println(err)
    return false
  }
  client := &http.Client{
    Timeout: time.Second * time.Duration(3),
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

  defer r.Body.Close()
	j := raftReq{}
	json.NewDecoder(r.Body).Decode(&j)
  if master.leader != j.Server && j.Server == master.votedFor {
    master.leader = j.Server
    fmt.Println("Leader is now", master.leader)
  }

	w.WriteHeader(http.StatusOK)
  resetElectionTimeout()
}

func voteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")

  defer r.Body.Close()
	j := raftReq{}
	json.NewDecoder(r.Body).Decode(&j)

  if master.candidate || master.votedFor != "" {
    w.WriteHeader(http.StatusLocked)
  } else {
    master.votedFor = j.Server
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
    master.votedFor = ""
    master.leader = ""
    <-master.electionTimer.C
    leaderElection()
  }
}


//---------------------API------------------------------------------------
