package main

import (
	"GoHttpRequestTaskProxy/internal/taskstore"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	_ "GoHttpRequestTaskProxy/docs"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type taskServer struct {
	store  *taskstore.TaskStore
	router *gin.Engine
}

func NewTaskServer() (*taskServer, error) {
	store, err := taskstore.New()
	if err != nil {
		return nil, err
	}
	return &taskServer{store: store}, nil
}

// Get all tasks
// @Summary Get all tasks
// @Schemes
// @Description A JSON array of tasks
// @Accept json
// @Produce json
// @Param status query string false "Status"
// @Param httpRequestCode query int false "HTTP Request Code"
// @Success 200 {array} taskstore.Task
// @Failure 500
// @Router /task/ [get]
func (ts *taskServer) getAllTasksHandler(c *gin.Context) {
	//parameter checking
	status := c.Query("status")
	if status != "done" && status != "in-progress" && status != "error" && status != "" {
		c.String(http.StatusBadRequest, "error: status can only be done, in-progress, error or empty string")
		return
	}
	var httpStatusCode *int
	httpStatusCodeString := c.Query("httpStatusCode")
	if httpStatusCodeString == "" {
		httpStatusCode = nil
	} else {
		ret, err := strconv.Atoi(httpStatusCodeString)
		httpStatusCode = &ret
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
	}

	allTasks, err := ts.store.GetAllTasks(status, httpStatusCode)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, allTasks)
}

// Delete all tasks
// @Summary Delete all tasks
// @Schemes
// @Description Deletes all tasks on the server. Requires authorization.
// @Accept json
// @Produce json
// @Success 200
// @Router /task/ [delete]
// @security BasicAuth
func (ts *taskServer) deleteAllTasksHandler(c *gin.Context) {
	err := ts.store.DeleteAllTasks()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, nil)
}

// RequestTask represents the request body for creating a task
// @Description Request body for creating a task
type RequestTask struct {
	Method  string            `json:"method"`  // The HTTP method (e.g., GET, POST)
	Url     string            `json:"url"`     // The URL of the third-party service
	Headers map[string]string `json:"headers"` // The headers to include in the request
	Body    string            `json:"body"`    // The body of the request (optional)
}

// Create a task
// @Summary Create a task
// @Schemes
// @Description Create a task on the server by providing the third-party serviceurl, method, headers and optionally a body. Returns a json containing the id of the task on success.
// @Accept json
// @Produce json
// @Param request body RequestTask true "Task request body"
// @Success 200
// @Router /task/ [post]
func (ts *taskServer) createTaskHandler(c *gin.Context) {

	var rt RequestTask
	if err := c.ShouldBindJSON(&rt); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	scheduledStartTime := time.Now()
	id, err := ts.store.CreateTask("in-progress", 202, make(map[string]string), 0, scheduledStartTime)

	if err != nil {
		ts.store.ChangeTask(id, "error", http.StatusInternalServerError, make(map[string]string), err.Error(), int64(len(err.Error())), scheduledStartTime, time.Now())
		c.JSON(http.StatusInternalServerError, gin.H{"Id": id})
		return
	}

	var req *http.Request

	if rt.Method == "GET" {
		req, err = http.NewRequest("GET", rt.Url, nil)
	} else {
		req, err = http.NewRequest(rt.Method, rt.Url, bytes.NewBuffer([]byte(rt.Body)))
		req.Header.Set("Content-Type", "application/json")
	}

	if err != nil {
		ts.store.ChangeTask(id, "error", http.StatusInternalServerError, make(map[string]string), err.Error(), int64(len(err.Error())), scheduledStartTime, time.Now())
		c.JSON(http.StatusInternalServerError, gin.H{"Id": id})
		return
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)

	headers := make(map[string]string)

	if err != nil {
		ts.store.ChangeTask(id, "error", http.StatusInternalServerError, make(map[string]string), err.Error(), int64(len(err.Error())), scheduledStartTime, time.Now())
		c.JSON(http.StatusInternalServerError, gin.H{"Id": id})
		return
	}

	for key, values := range resp.Header {

		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		ts.store.ChangeTask(id, "error", http.StatusInternalServerError, make(map[string]string), err.Error(), int64(len(err.Error())), scheduledStartTime, time.Now())
		c.JSON(http.StatusInternalServerError, gin.H{"Id": id})
		return
	}
	defer resp.Body.Close()

	ts.store.ChangeTask(id, "done", resp.StatusCode, headers, string(bodyBytes), resp.ContentLength, scheduledStartTime, time.Now())
	c.JSON(http.StatusOK, gin.H{"Id": id})
}

// Get a task by id
// @Summary Get a task by id
// @Schemes
// @Description A JSON task
// @Accept json
// @Produce json
// @Param id path int true "Task ID"
// @Success 200 {object} taskstore.Task
// @Router /task/{id} [get]
func (ts *taskServer) getTaskHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Params.ByName("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	task, err := ts.store.GetTask(id)
	if err != nil {
		c.String(http.StatusNotFound, err.Error())
		return
	}

	c.JSON(http.StatusOK, task)
}

// Delete a task with a specific id
// @Summary Delete a task with a specific id
// @Schemes
// @Description Returns 200 on success, 404 if the task does not exist and a bad request when the id is not a number.
// @Accept json
// @Produce json
// @Param id path int true "Task ID"
// @Success 200
// @Router /task/{id} [delete]
// @security BasicAuth
func (ts *taskServer) deleteTaskHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Params.ByName("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	if err = ts.store.DeleteTask(id); err != nil {
		c.String(http.StatusNotFound, err.Error())
	}
}

func setupServer() (*taskServer, error) {
	router := gin.Default()
	server, err := NewTaskServer()
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	accounts := gin.Accounts{"admin": "secret"}

	router.POST("/task/", server.createTaskHandler)
	router.GET("/task", server.getAllTasksHandler)
	router.GET("/task/", server.getAllTasksHandler)
	router.GET("/task/:id", server.getTaskHandler)
	router.DELETE("/task/", gin.BasicAuth(accounts), server.deleteAllTasksHandler)
	router.DELETE("/task/:id", gin.BasicAuth(accounts), server.deleteTaskHandler)

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	server.router = router
	return server, nil
}

// @title GoHttpRequestTaskProxy
// @version 1.0
// @description This is an API that sends requests to third party services and stores the responses in a tasks database
// @host localhost:8080
// @BasePath /
func main() {
	server, err := setupServer()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer server.store.CloseDatabasePool()

	server.router.Run(":8080")
}
