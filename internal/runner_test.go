package internal

import (
	"strings"
	"testing"
	"time"
)

func TestRunnerValidCommand(t *testing.T) {
	runner := newRunner("testRunnerValidCommand", "echo", "passString123", 10)

	err := runner.run()
	if err != nil {
		t.Error(err)
	}

	for runner.status == statusRunning {
		if runner.status == statusFailed || runner.status == statusTimeout {
			runner.kill()
			t.Error("Command failed")
		}
		time.Sleep(time.Second * 1)
	}

	if !strings.Contains(runner.Output, "passString123") {
		t.Error("Command doesn't contain expected output", runner.Output)
	}
}

func TestRunnerInvalidCommand(t *testing.T) {
	runner := newRunner("TestRunnerInvalidCommand", "nonexistent", "-c 12 -e", 10)

	err := runner.run()
	if err != nil {
		t.Fatal(err)
	}

	for runner.status == statusRunning {
		time.Sleep(time.Millisecond * 200)
	}

	if runner.status != statusFailed {
		t.Fatal("Unexpected status. Expected failed, got: ", runner.status)
	}
}

func TestRunnerCommandTimeout(t *testing.T) {
	runner := newRunner("timeoutCommand", "ping", "localhost -c 30", 2)

	err := runner.run()
	if err != nil {
		t.Fatal(err)
	}

	for runner.status == statusRunning {
		time.Sleep(time.Second)
	}

	if runner.status != statusTimeout {
		t.Fatal("Command didn't timeout. Current status is: ", runner.status)
	}
}

func TestRunnerCommandFailure(t *testing.T) {
	runner := newRunner("failedCommand", "cat", "absolutelyNonexistent", 5)

	err := runner.run()
	if err != nil {
		t.Fatal(err)
	}

	for runner.status == statusRunning {
		time.Sleep(time.Millisecond * 100)
	}

	if runner.status != statusFailed {
		t.Fatalf("Expected status: %s, got: %s", statusFailed, runner.status)
	}
}

func TestRunnerCommandKill(t *testing.T) {
	runner := newRunner("readingFromRunnerChannel", "ping", "localhost -c 10", 10)
	err := runner.run()
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second * 5)
	runner.kill()
	time.Sleep(time.Second)
	t.Log(runner.Duration)
	if runner.status != statusUserTerminated {
		t.Fatalf("Expected status %s, got: %s", statusUserTerminated, runner.status)
	}
}

func TestReadingFromRunnerChannelSingleLine(t *testing.T) {
	runner := newRunner("readingFromRunnerChannel", "echo", "chanoutbaby", 10)
	runnerChan := runner.registerClient("cli")
	outChan := make(chan string)

	go func() {
		outChan <- string(<-runnerChan)
	}()

	runner.run()
	for runner.status == statusRunning {
		time.Sleep(time.Millisecond * 100)
	}

	if runner.status != statusSuccess {
		t.Fatal("Expected succes status, got: ", runner.status)
	}

	if <-outChan != runner.Output {
		t.Fatal("Failed to get identical data from channel and struct")
	}
}

func TestReadingFromRunnerChannelLongDuration(t *testing.T) {
	runner := newRunner("readingFromRunnerChannel", "ping", "localhost -c 3", 10)
	runnerChan := runner.registerClient("cli")
	var chanOut string

	go func() {
		for msg := range runnerChan {
			chanOut += string(msg)
		}
	}()

	runner.run()
	for runner.status == statusRunning {
		time.Sleep(time.Millisecond * 100)
	}

	if runner.status != statusSuccess {
		t.Fatal("Expected succes status, got: ", runner.status)
	}

	if chanOut != runner.Output {
		t.Fatal("Failed to get identical data from channel and struct")
	}
}

func TestReadingFromRunnerChannelLongDurationTimeout(t *testing.T) {
	runner := newRunner("readingFromRunnerChannel", "ping", "localhost -c 10", 5)
	runnerChan := runner.registerClient("cli")
	var chanOut string

	go func() {
		for msg := range runnerChan {
			chanOut += string(msg)
		}
	}()

	runner.run()
	for runner.status == statusRunning {
		time.Sleep(time.Millisecond * 100)
	}

	if runner.status != statusTimeout {
		t.Fatalf("Expected status: %s, got: %s", statusSuccess, runner.status)
	}

	if chanOut != runner.Output {
		t.Fatalf("Failed to get identical data from channel and struct. Chan: %s Var: %s",
			chanOut, runner.Output)
	}
}
