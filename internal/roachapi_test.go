package internal

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// Following tests are not optimal as they are more of a integration ones
// Might need to replace with more granular ones.

func init() {
	os.Setenv("ROACH_TOKEN", "X")
	srv := New(":8981")
	go srv.Start()
	client := http.Client{}

	// Wait for our apiserver to come online before doing requests.
	// Server will be shut down automatically when tests end.
	// Be careful not to have another server running in background,
	// otherwise it will complain about port in use.
	for {
		healthReq, _ := http.NewRequest("GET", "http://localhost:8981/api/health", nil)
		resp, err := client.Do(healthReq)
		if resp.StatusCode == 200 && err == nil {
			break
		}
		time.Sleep(time.Second)
	}
}

func TestApiCreateRunner(t *testing.T) {

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

	// TODO - Add ROACH_TOKEN to requests once it's enabled
	req, err := http.NewRequest("POST", "http://localhost:8981/api/runner", bytes.NewBuffer(runnerJson))
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

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	jsonOut := make(map[string]string)
	json.Unmarshal(body, &jsonOut)

	if jsonOut["status"] != "OK" {
		t.Fatalf("Expected {\"status\": \"OK\"}, got: %s", jsonOut)
	}
}

func TestCompleteRunnerLifecycle(t *testing.T) {
	runnerJson, err := json.Marshal(
		&RunnerRequest{
			Name:    "TestApiRunnerLifecycle",
			Cmd:     "ping",
			Args:    "google.com -c 20",
			Timeout: 10,
		})

	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "http://localhost:8981/api/runner", bytes.NewBuffer(runnerJson))
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

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	jsonOut := make(map[string]string)
	json.Unmarshal(body, &jsonOut)

	if jsonOut["status"] != "OK" {
		t.Fatalf("Expected {\"status\": \"OK\"}, got: %s", jsonOut)
	}

	// Connect to SSE and wait for command to reach completion
	// by checking for event:done
	sseReq, err := http.NewRequest("GET", "http://localhost:8981/api/runner/TestApiRunnerLifecycle/stream", nil)
	sseReq.Header.Set("Cache-Control", "no-cache")
	sseReq.Header.Set("Accept", "text/event-stream")
	sseReq.Header.Set("Connection", "keep-alive")
	if err != nil {
		t.Fatal(err)
	}

	sseRes, err := client.Do(sseReq)
	if err != nil {
		t.Fatal(err)
	}
	defer sseRes.Body.Close()

	for {
		data := make([]byte, 1024)
		_, err := sseRes.Body.Read(data)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatal(err)
		}
	}

	// Delete runner after successful completion
	delReq, err := http.NewRequest("DELETE", "http://localhost:8981/api/runner/TestApiRunnerLifecycle", nil)
	if err != nil {
		t.Fatal(err)
	}

	delRes, err := client.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	defer delRes.Body.Close()

	if delRes.StatusCode != http.StatusOK {
		t.Fatalf("Wrong status code. Expected HTTP-200, got HTTP-%v", delRes.StatusCode)
	}

}
