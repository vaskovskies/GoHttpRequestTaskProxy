package taskserver

import (
	"GoHttpRequestTaskProxy/internal/taskstore"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	_ "GoHttpRequestTaskProxy/docs"

	"github.com/gin-gonic/gin"
	ginSwagger "github.com/swaggo/gin-swagger"

	swaggerFiles "github.com/swaggo/files"
)

type TaskServer struct {
	Store  *taskstore.TaskStore
	Router *gin.Engine
	Tasks  chan RequestTask
	Wg     sync.WaitGroup
	Srv    http.Server
}

func New() (*TaskServer, error) {
	store, err := taskstore.New()
	if err != nil {
		return nil, err
	}
	router := gin.Default()
	server := TaskServer{Store: store}

	accounts := gin.Accounts{"admin": "secret"}

	router.POST("/task", server.createTaskHandler)
	router.GET("/task", server.getAllTasksHandler)
	router.GET("/task/:id", server.getTaskHandler)
	router.DELETE("/task", gin.BasicAuth(accounts), server.deleteAllTasksHandler)
	router.DELETE("/task/:id", gin.BasicAuth(accounts), server.deleteTaskHandler)

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	server.Router = router
	server.Srv = http.Server{
		Addr:    ":8080",
		Handler: router.Handler(),
	}

	const numWorkers = 6
	server.Tasks = make(chan RequestTask, 1000000)

	for i := 1; i <= numWorkers; i++ {
		server.Wg.Add(1)
		go server.taskWorker(server.Tasks)
	}

	return &server, nil
}

func processTimeParameter(s string) (*time.Time, error) {
	const layout = time.RFC3339
	const layout_milli = "2006-01-02T15:04:05.000000Z07:00"
	var returnTime *time.Time
	if s == "" {
		returnTime = nil
	} else {
		var parsedTime time.Time
		parsedTime, err := time.Parse(layout_milli, s)
		if err != nil {
			parsedTime, err = time.Parse(layout, s)
			returnTime = &parsedTime
			if err != nil {
				return nil, err
			}
		}
		returnTime = &parsedTime
	}
	return returnTime, nil
}

func processTaskParameters(c *gin.Context) (string, *int, *time.Time, *time.Time, *time.Time, *time.Time, error) {

	status := c.Query("status")
	if status != taskstore.StatusDone && status != taskstore.StatusInProgress && status != taskstore.StatusError && status != "" {
		return "", nil, nil, nil, nil, nil, fmt.Errorf("error: status can only be done, in-progress, error or empty string")
	}

	var httpStatusCode *int
	httpStatusCodeString := c.Query("httpStatusCode")
	if httpStatusCodeString == "" {
		httpStatusCode = nil
	} else {
		ret, err := strconv.Atoi(httpStatusCodeString)
		httpStatusCode = &ret
		if err != nil {
			return status, nil, nil, nil, nil, nil, err
		}
	}

	min_scheduled_start_time, err := processTimeParameter(c.Query("minScheduledStartTime"))
	if err != nil {
		return status, httpStatusCode, nil, nil, nil, nil, err
	}

	max_scheduled_start_time, err := processTimeParameter(c.Query("maxScheduledStartTime"))
	if err != nil {
		return status, httpStatusCode, min_scheduled_start_time, nil, nil, nil, err
	}

	min_scheduled_end_time, err := processTimeParameter(c.Query("minScheduledEndTime"))
	if err != nil {
		return status, httpStatusCode, min_scheduled_start_time, max_scheduled_start_time, nil, nil, err
	}

	max_scheduled_end_time, err := processTimeParameter(c.Query("maxScheduledEndTime"))
	if err != nil {
		return status, httpStatusCode, min_scheduled_start_time, max_scheduled_start_time, min_scheduled_end_time, nil, err
	}

	return status, httpStatusCode, min_scheduled_start_time, max_scheduled_start_time, min_scheduled_end_time, max_scheduled_end_time, nil
}

// Get all tasks
// @Summary Get all tasks
// @Schemes
// @Description Returns a JSON array of tasks. Can be supplied parameters minScheduledStartTime, maxScheduledStartTime, minScheduledEndTime, maxScheduledEndTime, status and httpStatusCode to get tasks with those parameters. Date/Time format: YYYY-MM-DDTHH:MM:SS
// @Accept json
// @Produce json
// @Param status query string false "Status"
// @Param httpStatusCode query int false "HTTP Status Code"
// @Param minScheduledStartTime query string false "Minimum scheduled start time in format YYYY-MM-DDTHH:MM:SSZ"
// @Param maxScheduledStartTime query string false "Maximum scheduled start time in format YYYY-MM-DDTHH:MM:SSZ"
// @Param minScheduledEndTime query string false "Minimum scheduled end time in format YYYY-MM-DDTHH:MM:SSZ"
// @Param maxScheduledEndTime query string false "Maximum scheduled end time in format YYYY-MM-DDTHH:MM:SSZ"
// @Success 200 {array} taskstore.Task
// @Failure 500
// @Router /task [get]
func (ts *TaskServer) getAllTasksHandler(c *gin.Context) {
	status, httpStatusCode, minScheduledStartTime, maxScheduledStartTime, minScheduledEndTime, maxScheduledEndTime, err := processTaskParameters(c)

	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	if status == "" && httpStatusCode == nil && minScheduledStartTime == nil && maxScheduledStartTime == nil && minScheduledEndTime == nil && maxScheduledEndTime == nil {
		allTasks, err := ts.Store.GetAllTasks()
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, allTasks)
		return
	}

	allTasks, err := ts.Store.GetTasksWithFilter(status, httpStatusCode, minScheduledStartTime, maxScheduledStartTime, minScheduledEndTime, maxScheduledEndTime)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, allTasks)
}

// Delete all tasks
// @Summary Delete all tasks
// @Schemes
// @Description Deletes all tasks on the server. Requires authorization. Can be supplied parameters minScheduledStartTime, maxScheduledStartTime, minScheduledEndTime, maxScheduledEndTime, status and httpStatusCode to delete tasks with those parameters. Date/Time format: YYYY-MM-DDTHH:MM:SS
// @Accept json
// @Produce json
// @Param status query string false "Status"
// @Param httpStatusCode query int false "HTTP Status Code"
// @Param minScheduledStartTime query string false "Minimum scheduled start time in format YYYY-MM-DDTHH:MM:SSZ"
// @Param maxScheduledStartTime query string false "Maximum scheduled start time in format YYYY-MM-DDTHH:MM:SSZ"
// @Param minScheduledEndTime query string false "Minimum scheduled end time in format YYYY-MM-DDTHH:MM:SSZ"
// @Param maxScheduledEndTime query string false "Maximum scheduled end time in format YYYY-MM-DDTHH:MM:SSZ"
// @Success 200
// @Router /task [delete]
// @security BasicAuth
func (ts *TaskServer) deleteAllTasksHandler(c *gin.Context) {
	status, httpStatusCode, minScheduledStartTime, maxScheduledStartTime, minScheduledEndTime, maxScheduledEndTime, err := processTaskParameters(c)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	if status == "" && httpStatusCode == nil && minScheduledStartTime == nil && maxScheduledStartTime == nil && minScheduledEndTime == nil && maxScheduledEndTime == nil {
		ts.Store.DeleteAllTasks()
		c.JSON(http.StatusOK, nil)
		return
	}
	err = ts.Store.DeleteTasksWithFilter(status, httpStatusCode, minScheduledStartTime, maxScheduledStartTime, minScheduledEndTime, maxScheduledEndTime)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, nil)
}

// RequestBody represents the request body for creating a task
// @Description Request body for creating a task
type RequestBody struct {
	Method  string            `json:"method"`  // The HTTP method (e.g., GET, POST)
	Url     string            `json:"url"`     // The URL of the third-party service
	Headers map[string]string `json:"headers"` // The headers to include in the request
	Body    string            `json:"body"`    // The body of the request (optional)
}

type RequestTask struct {
	Id          int64
	RequestBody RequestBody
}

func (ts *TaskServer) taskWorker(tasks <-chan RequestTask) {
	defer ts.Wg.Done()
	for task := range tasks {
		var req *http.Request
		var reqBodyBuf io.Reader = http.NoBody
		if task.RequestBody.Body != "" {
			reqBodyBuf = bytes.NewBuffer([]byte(task.RequestBody.Body))
		}
		req, err := http.NewRequest(task.RequestBody.Method, task.RequestBody.Url, reqBodyBuf)
		req.Header.Set("Content-Type", "application/json")

		if err != nil {
			errorMessage := err.Error()
			err := ts.Store.ChangeTask(task.Id, taskstore.StatusError, http.StatusInternalServerError, make(map[string]string), &errorMessage, int64(len(err.Error())), time.Now())
			if err != nil {
				log.Println("couldn't change task: ", task.Id)
			}
			continue
		}

		client := &http.Client{
			Timeout: 5 * time.Second,
		}

		resp, err := client.Do(req)

		if err != nil {
			errorMessage := err.Error()
			err := ts.Store.ChangeTask(task.Id, taskstore.StatusError, http.StatusInternalServerError, make(map[string]string), &errorMessage, int64(len(err.Error())), time.Now())
			if err != nil {
				log.Println("couldn't change task: ", task.Id)
			}
			continue
		}

		headers := make(map[string]string)

		for key, values := range resp.Header {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}

		responseBodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			errorMessage := err.Error()
			err := ts.Store.ChangeTask(task.Id, taskstore.StatusError, http.StatusInternalServerError, make(map[string]string), &errorMessage, int64(len(err.Error())), time.Now())
			if err != nil {
				log.Println("couldn't change task: ", task.Id)
			}
			continue
		}
		resp.Body.Close()
		responseBodyString := string(responseBodyBytes)
		err = ts.Store.ChangeTask(task.Id, taskstore.StatusDone, resp.StatusCode, headers, &responseBodyString, resp.ContentLength, time.Now())
		if err != nil {
			log.Println("couldn't change task: ", task.Id)
		}
	}
}

// Create a task
// @Summary Create a task
// @Schemes
// @Description Create a task on the server by providing the third-party service url, method, headers and a body. Returns a json containing the id of the task on success.
// @Accept json
// @Produce json
// @Param request body RequestBody true "Task request body"
// @Success 200
// @Router /task [post]
func (ts *TaskServer) createTaskHandler(c *gin.Context) {

	var rt RequestBody
	if err := c.ShouldBindJSON(&rt); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	requestBodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, "couldn't read request body")
		return
	}
	defer c.Request.Body.Close()

	scheduledStartTime := time.Now()
	if rt.Headers == nil {
		rt.Headers = make(map[string]string)
	}
	id, err := ts.Store.CreateTask(taskstore.StatusInProgress, 202, rt.Headers, string(requestBodyBytes), 0, scheduledStartTime)

	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	task := RequestTask{Id: id, RequestBody: rt}
	select {
	case ts.Tasks <- task:
		c.JSON(http.StatusOK, gin.H{"Id": id})
	default:
		errorMessage := "task queue is full"
		err := ts.Store.ChangeTask(id, taskstore.StatusError, http.StatusInternalServerError, make(map[string]string), &errorMessage, -1, time.Now())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"Id": id})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"Id": id})
	}

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
func (ts *TaskServer) getTaskHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Params.ByName("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	task, err := ts.Store.GetTask(id)
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
func (ts *TaskServer) deleteTaskHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Params.ByName("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	if err = ts.Store.DeleteTask(id); err != nil {
		c.String(http.StatusNotFound, err.Error())
	}
}
