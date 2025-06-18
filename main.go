package main

import (
	"GoHttpRequestTaskProxy/internal/taskstore"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type taskServer struct {
	store *taskstore.TaskStore
}

func NewTaskServer() (*taskServer, error) {
	store, err := taskstore.New()
	if err != nil {
		return nil, err
	}
	return &taskServer{store: store}, nil
}

func (ts *taskServer) getAllTasksHandler(c *gin.Context) {
	allTasks, err := ts.store.GetAllTasks()
	if err != nil {
		return
	}
	c.JSON(http.StatusOK, allTasks)
}

func (ts *taskServer) deleteAllTasksHandler(c *gin.Context) {
	ts.store.DeleteAllTasks()
}

func (ts *taskServer) createTaskHandler(c *gin.Context) {
	type RequestTask struct {
		Method  string            `json:"method"`
		Url     string            `json:"url"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
	}

	var rt RequestTask
	if err := c.ShouldBindJSON(&rt); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	scheduledStartTime := time.Now()
	id, err := ts.store.CreateTask("in_process", 202, make(map[string]string), 0, scheduledStartTime)

	if err != nil {
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
		ts.store.ChangeTask(id, "error", http.StatusInternalServerError, make(map[string]string), "", 0, scheduledStartTime, time.Now())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)

	headers := make(map[string]string)

	if err != nil {
		ts.store.ChangeTask(id, "error", http.StatusInternalServerError, headers, "", 0, scheduledStartTime, time.Now())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		ts.store.ChangeTask(id, "error", http.StatusInternalServerError, headers, "", 0, scheduledStartTime, time.Now())
		return
	}
	defer resp.Body.Close()

	ts.store.ChangeTask(id, "done", resp.StatusCode, headers, string(bodyBytes), resp.ContentLength, scheduledStartTime, time.Now())
	c.JSON(http.StatusOK, gin.H{"Id": id})
}

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

func main() {
	router := gin.Default()
	server, err := NewTaskServer()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer server.store.CloseDatabasePool()

	accounts := gin.Accounts{"admin": "secret"}

	router.POST("/task/", server.createTaskHandler)
	router.GET("/task/", server.getAllTasksHandler)
	router.GET("/task/:id", server.getTaskHandler)
	router.DELETE("/task/", gin.BasicAuth(accounts), server.deleteAllTasksHandler)
	router.DELETE("/task/:id", gin.BasicAuth(accounts), server.deleteTaskHandler)

	router.Run(":8080")
}
