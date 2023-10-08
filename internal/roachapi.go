package internal

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type roach struct {
	listenAddr string
	authToken  string
	runners    map[string]*runner
	runnerLock sync.Mutex
}

func New(listen string) *roach {
	authToken, present := os.LookupEnv("ROACH_TOKEN")
	if !present || authToken == "" {
		log.Fatal("ROACH_TOKEN not set or empty.")
	}

	return &roach{
		listenAddr: listen,
		authToken:  authToken,
		runners:    make(map[string]*runner),
	}
}

func (r *roach) Start() {
	router := gin.Default()

	// Health check endpoint
	router.GET("/api/health", r.health)

	// Runner related endpoints
	router.GET("/api/runners", r.readRunners)
	router.POST("/api/runner", r.addRunner)
	router.GET("/api/runner/:name", r.readRunner)
	router.GET("/api/runner/:name/output", r.readRunnerOutput)
	router.GET("/api/runner/:name/stream", r.runnerStreamEvents)
	router.DELETE("/api/runner/:name", r.deleteRunner)

	// Static content provisioning
	router.Static("/static/", "/tmp/static")

	// Dashboard related endpoints
	router.GET("/dashboard", nil)
	router.GET("/dashboard/liveout", nil)
	router.GET("/dashboard/runners", nil)
	router.GET("/dashboard/clean", nil)

	err := router.Run(r.listenAddr)
	if err != nil {
		log.Fatal(err)
	}
}

// Health endpoint for livecheck
func (r *roach) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "running"})
}

func (r *roach) clean(c *gin.Context) {
	c.Writer.WriteString("")
}

func (r *roach) serveDashboard(c *gin.Context) {
	tmpl := template.Must(template.ParseFiles("/tmp/roach/static/dashboard.html"))
	tmpl.Execute(c.Writer, r.runners)
}

func (r *roach) serveRunners(c *gin.Context) {
	tmpl := template.Must(template.ParseFiles(
		"/tmp/static/runners.html"))
	tmpl.Execute(c.Writer, r.runners)
}

func (r *roach) serveConsoleOutElement(c *gin.Context) {
	param, exists := c.GetQuery("runnerName")
	if exists {
		tmpl := template.Must(template.ParseFiles("/tmp/static/output.html"))
		tmpl.Execute(c.Writer, r.runners[param])
		return
	} else {
		c.JSON(http.StatusNotFound, gin.H{"status": "not found"})
	}
}

func (r *roach) addRunner(c *gin.Context) {
	var rReq RunnerRequest
	if err := c.BindJSON(&rReq); err != nil {
		log.Fatal(err)
	}

	r.runnerLock.Lock()
	if r.runners[rReq.Name] != nil {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf(
				"Resource %s already exists. Name must be unique", rReq.Name,
			),
		})
		r.runnerLock.Unlock()
		return
	}
	r.runnerLock.Unlock()

	log.Printf("adding new worker %s\n", rReq.Name)
	log.Printf("worker details: Command: %s\tArgs: %s\tTimeout: %ds\n",
		rReq.Cmd, rReq.Args, rReq.Timeout)

	runner := newRunner(rReq.Name, rReq.Cmd, rReq.Args, rReq.Timeout)
	err := runner.run()
	if err != nil {
		log.Fatal(err)
	}

	r.runnerLock.Lock()
	r.runners[rReq.Name] = runner
	r.runnerLock.Unlock()

	c.JSON(http.StatusCreated, gin.H{"status": "OK"})
}

// Handle for deleting a runner
func (r *roach) deleteRunner(c *gin.Context) {
	name := c.Param("name")
	r.runnerLock.Lock()
	defer r.runnerLock.Unlock()
	runner := r.runners[name]

	if runner == nil {
		c.AbortWithStatusJSON(
			http.StatusNotFound, gin.H{
				"error": fmt.Sprintf("resource %v not found", name),
			})
		return
	}
	log.Println("Deleting runner", name)
	runner.kill()
	delete(r.runners, name)

	c.JSON(http.StatusOK, gin.H{
		"status": fmt.Sprintf("resource %v deleted successfully", name),
	})
}

func (r *roach) readRunners(c *gin.Context) {
	var runnerList [][]byte

	r.runnerLock.Lock()
	for _, runner := range r.runners {
		runnerJson, err := json.Marshal(runner)
		if err != nil {
			log.Fatalf("can't marshall runner %s to json", runner.RunnerName)
		}
		runnerList = append(runnerList, runnerJson)
	}
	r.runnerLock.Unlock()

	if len(runnerList) < 1 {
		c.JSON(http.StatusOK, gin.H{"status": "worker list empty"})
		return
	}
	c.JSON(http.StatusOK, runnerList)
}

func (r *roach) readRunner(c *gin.Context) {
	name := c.Param("name")

	r.runnerLock.Lock()
	runner := r.runners[name]
	if runner == nil {
		c.AbortWithStatusJSON(
			http.StatusNotFound, gin.H{
				"error": fmt.Sprintf("resource %s not found", name),
			})
		return
	}
	r.runnerLock.Unlock()
	c.IndentedJSON(http.StatusOK, runner)
}

func (r *roach) readRunnerOutput(c *gin.Context) {
	name := c.Param("name")

	r.runnerLock.Lock()
	runner := r.runners[name]

	if runner == nil {
		c.AbortWithStatusJSON(
			http.StatusNotFound, gin.H{
				"error": fmt.Sprintf("resource %s not found", name),
			})
		r.runnerLock.Unlock()
		return
	}
	r.runnerLock.Unlock()
	c.String(http.StatusOK, "%s", runner.output)
}

// Handle for providing clients with stream of combined stdout/stderr
func (r *roach) runnerStreamEvents(c *gin.Context) {
	name := c.Param("name")
	clientId := uuid.New().String()

	r.runnerLock.Lock()
	runner := r.runners[name]

	if runner == nil {
		c.SSEvent("close", "Close connection")
		return
	}

	log.Println("New subscriber ", clientId)
	channel := runner.registerClient(clientId)
	closeNotify := c.Writer.CloseNotify()
	r.runnerLock.Unlock()

	c.Stream(func(w io.Writer) bool {
		msg := <-channel

		select {
		// Listen on close notify channel to correctly
		// Unsubscribe in case of client terminating
		case end := <-closeNotify:
			log.Println("client disconnected ", end)
			runner.unregisterClient(clientId)
			return false
		default:
			if runner.Status != statusRunning {
				c.SSEvent("done", "true")
				return true
			}
			c.SSEvent("message", string(msg))
			return true
		}
	})
}
