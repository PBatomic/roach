package internal

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

func spawnApiServer() {
	os.Setenv("ROACH_TOKEN", "X")
	srv := New(":8081")
	go srv.Start()
	time.Sleep(time.Second * 3)
}

func TestApiCreateRunner(t *testing.T) {
	spawnApiServer()
	runnerJson, err := json.Marshal(
		&RunnerRequest{
			Name:    "TestApiCreateRunner",
			Cmd:     "ping",
			Args:    "google.com -c 5",
			Timeout: 10,
		})

	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "http://localhost:8081/api/runner", bytes.NewBuffer(runnerJson))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	if err != nil {
		t.Fatal(err)
	}

	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	
	defer res.Body.Close()

}
