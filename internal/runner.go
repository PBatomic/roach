package internal

import (
	"context"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Runner status to simplify passing info to apiserver
const (
	statusReady          = "ready"
	statusRunning        = "running"
	statusSuccess        = "success"
	statusFailed         = "failed"
	statusTimeout        = "timeout"
	statusUserTerminated = "terminated"
)

type RunnerRequest struct {
	Name    string `json:"name,omitempty"`
	Cmd     string `json:"cmd,omitempty"`
	Args    string `json:"args,omitempty"`
	Timeout int    `json:"timeout,omitempty"`
}

type runner struct {
	RunnerName string    `json:"runner_name"`
	CmdString  string    `json:"cmd_string"`
	Args       string    `json:"args"`
	Timeout    int       `json:"timeout"`
	StartTime  time.Time `json:"start_time"`
	StopTime   time.Time `json:"stop_time,omitempty"`
	Duration   float64   `json:"duration"`
	ErrorMsg   string    `json:"error_msg"`
	Status     string    `json:"status,omitempty"`

	output        string
	cmd           *exec.Cmd
	cancel        context.CancelFunc
	streamManager *streamManager
	managerLock   sync.Mutex
	ctx           context.Context
}

// Writer implementation for CMD outputs.
// Writes into struct var as well as to streamManager
func (r *runner) Write(p []byte) (int, error) {
	r.output += string(p)
	r.managerLock.Lock()
	r.streamManager.Write(p)
	r.managerLock.Unlock()

	return len(p), nil
}

// GetStatusColor is used to provide color to tailwindcss div class
func (r *runner) GetStatusColor() string {
	switch r.Status {
	case statusSuccess:
		return "bg-lime-500"
	case statusFailed:
		return "bg-rose-600"
	case statusTimeout:
		return "bg-rose-400"
	case statusUserTerminated:
		return "bg-slate-600"
	case statusRunning:
		return "bg-sky-600"
	}
	return "bg-transparent"
}

func newRunner(name string, cmd string, args string, timeout int) *runner {
	runner := &runner{
		RunnerName: name,
		CmdString:  cmd,
		Args:       args,
		Timeout:    timeout,
		output:     "",
		Status:     statusReady,
	}

	runner.streamManager = newStreamManager()

	return runner
}

// Runs command as configured with newRunner func.
func (w *runner) run() error {
	// Context is needed to provide timeout option
	log.Printf("running %s with command: %s %s", w.RunnerName, w.CmdString, w.Args)
	w.ctx, w.cancel = context.WithTimeout(context.Background(),
		time.Duration(w.Timeout*int(time.Second)))
	w.cmd = exec.CommandContext(w.ctx, w.CmdString,
		strings.Split(w.Args, " ")...)

	// Setting to custom writers to get formatted output
	// saved to our struct
	w.cmd.Stdout = w
	w.cmd.Stderr = w

	// Get start time and start command using nonblocking cmd.Start()
	w.StartTime = time.Now()
	w.cmd.Start()
	w.Status = statusRunning

	// Wait for cmd to finish in goroutine
	// Sets the finished flag once done
	// If timeout is reached, we make sure we set the status to statusTimeout
	go func() {
		err := w.cmd.Wait()
		w.StopTime = time.Now()
		w.Duration = time.Since(w.StartTime).Seconds()

		w.managerLock.Lock()
		w.streamManager.CloseManager()
		w.managerLock.Unlock()

		if w.Status == statusUserTerminated {
			return
		}

		w.Status = statusSuccess

		if err != nil {
			if w.ctx.Err() == context.DeadlineExceeded {
				w.Status = statusTimeout
			} else {
				w.Status = statusFailed
			}
			w.ErrorMsg += err.Error()
		}
	}()
	return nil
}

// Stream registers subscribes client to stream manager
// and returns channel with combined stdout/stderr output
func (w *runner) registerClient(id string) chan []byte {
	return w.streamManager.Subscribe(id)
}

func (w *runner) unregisterClient(id string) {
	w.streamManager.Unsubscribe(id)
}

// End process using cancler
func (w *runner) kill() error {
	if w.Status == statusRunning {
		err := w.cmd.Process.Kill()
		if err != nil {
			return err
		}
	}
	w.Status = statusUserTerminated

	return nil
}
