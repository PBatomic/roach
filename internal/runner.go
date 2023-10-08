package internal

import (
	"context"
	"log"
	"os/exec"
	"strings"
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
	ctx           context.Context
}

// Writer implementation for CMD outputs.
// Writes into struct var as well as to streamManager
func (r *runner) Write(p []byte) (int, error) {
	r.output += string(p)
	r.streamManager.Write(p)
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
func (r *runner) run() error {
	// Context is needed to provide timeout option
	log.Printf("running %s with command: %s %s", r.RunnerName, r.CmdString, r.Args)
	r.ctx, r.cancel = context.WithTimeout(context.Background(),
		time.Duration(r.Timeout*int(time.Second)))
	r.cmd = exec.CommandContext(r.ctx, r.CmdString,
		strings.Split(r.Args, " ")...)

	// Setting to custom writers to get formatted output
	// saved to our struct
	r.cmd.Stdout = r
	r.cmd.Stderr = r

	// Get start time and start command using nonblocking cmd.Start()
	r.StartTime = time.Now()
	r.cmd.Start()
	r.Status = statusRunning

	// Wait for cmd to finish in goroutine
	// Sets the finished flag once done
	// If timeout is reached, we make sure we set the status to statusTimeout
	go func() {
		err := r.cmd.Wait()
		r.StopTime = time.Now()
		r.Duration = time.Since(r.StartTime).Seconds()
		r.streamManager.CloseManager()

		if r.Status == statusUserTerminated {
			return
		}

		r.Status = statusSuccess

		if err != nil {
			if r.ctx.Err() == context.DeadlineExceeded {
				r.Status = statusTimeout
			} else {
				r.Status = statusFailed
			}
			r.ErrorMsg += err.Error()
		}
	}()
	return nil
}

// Stream registers subscribes client to stream manager
// and returns channel with combined stdout/stderr output
func (r *runner) registerClient(id string) chan []byte {
	return r.streamManager.Subscribe(id)
}

func (r *runner) unregisterClient(id string) {
	r.streamManager.Unsubscribe(id)
}

// End process using cancler
func (r *runner) kill() error {
	if r.Status == statusRunning {
		err := r.cmd.Process.Kill()
		if err != nil {
			return err
		}
	}
	r.Status = statusUserTerminated

	return nil
}
