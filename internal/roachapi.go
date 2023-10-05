package internal

import (
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type roach struct {
	listenAddr string
	authToken  string
}

func New(listen string) *roach {
	authToken, present := os.LookupEnv("ROACH_TOKEN")
	if !present || authToken == "" {
		log.Fatal("ROACH_TOKEN not set or empty.")
	}

	return &roach{
		listenAddr: listen,
		authToken:  authToken,
	}
}

func (r *roach) Start() {
	router := gin.Default()

	// Json api handlers
	router.GET("/api/health", nil)

	// Runner related endpoints
	router.POST("/api/runner", nil)
	router.GET("/api/runner/:name", nil)
	router.GET("/api/runner/:name/output", nil)
	router.GET("/api/runner/:name/stream", nil)
	router.DELETE("/api/runner/:name", nil)
	router.GET("/api/runner", nil)

	// Static content provisioning
	router.Static("/static/", "/tmp/static")

	// Dashboard related endpoints
	router.GET("/dashboard", nil)
	router.GET("/dashboard/liveout", nil)
	router.GET("/dashboard/workerlist", nil)
	router.GET("/dashboard/clean", nil)

	err := router.Run()
	if err != nil {
		log.Fatal(err)
	}
}

// Health endpoint for livecheck
func (r *roach) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "running"})
}

// Element clearing hack
func (r *roach) clean(c *gin.Context) {
	c.Writer.WriteString("")
}

func (r *roach) serveDashboard(c *gin.Context) {
	tmpl := template.Must(template.ParseFiles("/tmp/roach/static/dashboard.html"))
}
