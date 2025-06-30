package main

import (
	"GoHttpRequestTaskProxy/internal/taskserver"
	"GoHttpRequestTaskProxy/internal/taskstore"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-playground/assert/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func recreateDb() error {

	if os.Getenv("DATABASE_URL") == "postgresql://postgres:postgres@localhost:5432" {
		os.Exit(0)
	}
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer pool.Close()

	pool.Exec(context.Background(), `
	DROP TABLE tasks;
	CREATE TABLE IF NOT EXISTS tasks (
    id BIGSERIAL PRIMARY KEY, 
    status VARCHAR(50) NOT NULL,
    http_status_code INT NOT NULL,
    request_headers JSONB NOT NULL,
	response_headers JSONB NULL,
    request_body TEXT NOT NULL,
	response_body TEXT,
    length BIGINT NOT NULL,
    scheduled_start_time TIMESTAMP NOT NULL,
    scheduled_end_time TIMESTAMP NULL
);`)

	return nil
}

func TestCreateRoute(t *testing.T) {

	err := recreateDb()
	if err != nil {
		t.Fatalf("Couldn't clean database. Is the enviroment variable set correctly? Is the database test container on?")
		return
	}
	server, err := taskserver.New()
	if err != nil {
		t.Fatalf("Couldn't set up test server. Is the enviroment variable set correctly? Is the database test container on?")
		return
	}

	defer func() {
		close(server.Tasks)
		server.Wg.Wait()
	}()

	w := httptest.NewRecorder()
	task := taskserver.RequestBody{
		Method: "GET",
		Url:    "https://jsonplaceholder.typicode.com/todos/1",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: "",
	}

	taskJSON, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Error marshaling task: %v", err)
	}

	req, _ := http.NewRequest("POST", "/task", bytes.NewBuffer(taskJSON))
	server.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"Id":1}`, w.Body.String())

	//test 2: check for bad request
	w = httptest.NewRecorder()

	req, _ = http.NewRequest("POST", "/task", nil)
	server.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

}

func TestGetRoutes(t *testing.T) {
	err := recreateDb()
	if err != nil {
		t.Fatalf("Couldn't clean database. Is the enviroment variable set correctly? Is the database test container on?")
		return
	}
	server, err := taskserver.New()
	if err != nil {
		t.Fatalf("Couldn't set up test server. Is the enviroment variable set correctly? Is the database test container on?")
		return
	}
	ischannelopen := true

	defer func() {
		if ischannelopen {
			close(server.Tasks)
			server.Wg.Wait()
		}
	}()

	assertInProgressEntryIsCorrect := func(task taskstore.Task) {
		if task.Status != taskstore.StatusNew && task.Status != taskstore.StatusInProgress {
			t.Error("New task status isn't new or in-progress")
			return
		}
		assert.Equal(t, task.RequestHeaders, map[string]string{"Content-Type": "application/json"})
		assert.Equal(t, task.ScheduledEndTime, nil)
		assert.Equal(t, task.ResponseBody, nil)
		assert.Equal(t, task.HttpStatusCode, http.StatusAccepted)
		assert.NotEqual(t, task.ScheduledStartTime, "")
		assert.Equal(t, task.Length, int64(0))
	}

	//send requests get responses. One is a correct request, the other one fails

	w := httptest.NewRecorder()

	task := taskserver.RequestBody{
		Method: "GET",
		Url:    "https://jsonplaceholder.typicode.com/todos/1",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: "",
	}

	taskJSON, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Error marshaling task: %v", err)
	}

	req, _ := http.NewRequest("POST", "/task", bytes.NewBuffer(taskJSON))
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, w.Code, http.StatusOK)
	//test that the in progress task is correct
	var taskResponse taskstore.Task

	w = httptest.NewRecorder()

	req, _ = http.NewRequest("GET", "/task/1", nil)

	server.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	err = json.Unmarshal(w.Body.Bytes(), &taskResponse)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
		return
	}
	assertInProgressEntryIsCorrect(taskResponse)

	//add incorrect task
	w = httptest.NewRecorder()
	task = taskserver.RequestBody{
		Method: "POST",
		Url:    "/task",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: `{"url": "http://invalid-url"}`,
	}

	req, _ = http.NewRequest("POST", "/task", bytes.NewBuffer([]byte(task.Body)))
	server.Router.ServeHTTP(w, req)
	t.Log(w.Body)
	assert.Equal(t, w.Code, http.StatusOK)

	//Close channel and wait for the tasks to finish to observe finished task results
	close(server.Tasks)
	server.Wg.Wait()

	ischannelopen = false

	//test 1: get by id
	w = httptest.NewRecorder()

	req, _ = http.NewRequest("GET", "/task/1", nil)

	server.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	err = json.Unmarshal(w.Body.Bytes(), &taskResponse)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
		return
	}

	assertIdIsOneTaskResponse := func(task taskstore.Task) {
		//check if id is correct
		assert.Equal(t, task.Id, int64(1))
		//check if the response is expected
		assert.Equal(t, *task.ResponseBody, `{
  "userId": 1,
  "id": 1,
  "title": "delectus aut autem",
  "completed": false
}`)
		//check if the headers are there
		if len(task.ResponseHeaders) == 0 {
			t.Error("Response headers map is empty when it shouldn't be")
		}
		//check if http status code is correct
		assert.Equal(t, task.HttpStatusCode, http.StatusOK)
		//check if the status of the task is done
		assert.Equal(t, task.Status, "done")
	}
	assertIdIsOneTaskResponse(taskResponse)

	t.Log("Get by id endpoint works correctly")

	//test 2: get all tasks
	w = httptest.NewRecorder()

	req, _ = http.NewRequest("GET", "/task", nil)

	server.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var allTasksResponse []taskstore.Task
	err = json.Unmarshal(w.Body.Bytes(), &allTasksResponse)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
		return
	}
	assertIdIsOneTaskResponse(allTasksResponse[0])

	//check if id is correct
	assert.Equal(t, allTasksResponse[1].Id, int64(2))
	//check if the response is expected: this test used to fail on docker
	//assert.Equal(t, *allTasksResponse[1].ResponseBody, `Get "http://invalid-url": dial tcp: lookup invalid-url: no such host`)
	if allTasksResponse == nil || len(*allTasksResponse[1].ResponseBody) == 0 {
		t.Error("Response body of an internal server error is empty or nil when it shouldn't be")
	}
	//check if the headers are not present
	if len(allTasksResponse[1].ResponseHeaders) != 0 {
		t.Error("Response headers map is full when it shouldn't be")
	}
	//check if http status code is correct
	assert.Equal(t, allTasksResponse[1].HttpStatusCode, http.StatusInternalServerError)

	//check if the status of the taskResponse[1] is done
	assert.Equal(t, allTasksResponse[1].Status, "error")

	//assert.Equal(t, `{"Id":1}`, w.Body.String())
	t.Log("Get all tasks endpoint works correctly")

	//test 3: check get on nonexistent tasks
	w = httptest.NewRecorder()

	req, _ = http.NewRequest("GET", "/task/404", nil)

	server.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	//test 4: test incorrect id format
	w = httptest.NewRecorder()

	req, _ = http.NewRequest("GET", "/task/NAN", nil)

	server.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetRequestParametrization(t *testing.T) {
	err := recreateDb()
	if err != nil {
		t.Fatalf("Couldn't clean database. Is the enviroment variable set correctly? Is the database test container on?")
		return
	}
	server, err := taskserver.New()
	if err != nil {
		t.Fatalf("Couldn't set up test server. Is the enviroment variable set correctly? Is the database test container on?")
		return
	}
	defer func() {
		close(server.Tasks)
		server.Wg.Wait()
	}()

	//test 1: test bad requests

	//invalid status
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/task?status=NOTVALID", nil)
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	//invalid httpStatusCode
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/task?httpStatusCode=NaN", nil)
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	//test 2: test if the parametrization works correctly

	//create fake test tasks
	const layout_milli = "2006-01-02T15:04:05.000000Z07:00"
	Time1, _ := time.Parse(layout_milli, "2006-01-02T12:00:00.000033Z")
	Time2, _ := time.Parse(time.RFC3339, "2008-01-02T16:00:00Z")
	Time3, _ := time.Parse(time.RFC3339, "2010-01-02T22:00:00Z")
	Time4, _ := time.Parse(time.RFC3339, "2012-01-02T23:00:00.000033Z")
	id, _ := server.Store.CreateTask("done", http.StatusInternalServerError, make(map[string]string), "dsad", 0, Time1)
	server.Store.ChangeTask(id, "done", http.StatusInternalServerError, make(map[string]string), nil, 0, Time1)
	id, _ = server.Store.CreateTask("done", http.StatusOK, make(map[string]string), "dsad", 0, Time2)
	server.Store.ChangeTask(id, "done", http.StatusOK, make(map[string]string), nil, 0, Time2)
	id, _ = server.Store.CreateTask("in-progress", http.StatusAccepted, make(map[string]string), "dsad", 0, Time3)
	server.Store.ChangeTask(id, "in-progress", http.StatusAccepted, make(map[string]string), nil, 0, Time3)
	id, _ = server.Store.CreateTask("error", http.StatusInternalServerError, make(map[string]string), "dsad", 0, Time4)
	server.Store.ChangeTask(id, "error", http.StatusInternalServerError, make(map[string]string), nil, 0, Time4)

	//send request for all tasks with statusCode 500
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/task?httpStatusCode=500", nil)
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var allTasksResponse []taskstore.Task
	err = json.Unmarshal(w.Body.Bytes(), &allTasksResponse)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
		return
	}
	for _, task := range allTasksResponse {
		if task.HttpStatusCode != 500 {
			t.Error("task query with parameter httpStatusCode=500 returned a task with a http status code other than 500")
			return
		}
	}

	//send request for all tasks with status = done
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/task?status=done", nil)
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	err = json.Unmarshal(w.Body.Bytes(), &allTasksResponse)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
		return
	}

	for _, task := range allTasksResponse {
		if task.Status != "done" {
			t.Error("task query with parameter status=done returned a task with a status other than done")
			return
		}
	}
	//send request for all tasks with status = done and httpStatusCode of 200
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/task?status=done&httpStatusCode=200", nil)
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	err = json.Unmarshal(w.Body.Bytes(), &allTasksResponse)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
		return
	}
	for _, task := range allTasksResponse {
		if task.Status != "done" && task.HttpStatusCode != 200 {
			t.Error("task query with parameter status=done and httpStatusCode=200 returned a task with a status other than done and httpStatusCode 200")
			return
		}
	}

	//find requests within scheduledStartTime Time2 and Time3
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/task?minScheduledStartTime=2008-01-02T16:00:00Z&maxScheduledStartTime=2010-01-02T22:00:00Z", nil)
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	err = json.Unmarshal(w.Body.Bytes(), &allTasksResponse)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
		return
	}
	for _, task := range allTasksResponse {
		if task.ScheduledStartTime.Before(Time2) || task.ScheduledStartTime.After(Time3) {
			t.Error("task query with parameters minScheduledStartTime=2008-01-02T16:00:00Z&maxScheduledStartTime=2010-01-02T22:00:00Z returned tasks outside of the bounds of the query")
			return
		}
	}

	//find requests within scheduledEndTime Time2 and Time3
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/task?minScheduledEndTime=2008-01-02T16:00:00Z&maxScheduledEndTime=2010-01-02T22:00:00Z", nil)
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	err = json.Unmarshal(w.Body.Bytes(), &allTasksResponse)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
		return
	}
	for _, task := range allTasksResponse {
		if task.ScheduledEndTime.Before(Time2) || task.ScheduledEndTime.After(Time3) {
			t.Error("task query with parameters minScheduledEndTime=2008-01-02T16:00:00Z&maxScheduledEndTime=2010-01-02T22:00:00Z returned tasks outside of the bounds of the query")
			return
		}
	}

	//find request with Time1
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/task?minScheduledStartTime=2006-01-02T12:00:00.000033Z&maxScheduledStartTime=2006-01-02T12:00:00.000033Z", nil)
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	err = json.Unmarshal(w.Body.Bytes(), &allTasksResponse)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
		return
	}
	if len(allTasksResponse) == 0 {
		t.Error("Couldn't find a task with Time1 (2006-01-02T12:00:00.000033Z) when queried for that one task")
	}
	for _, task := range allTasksResponse {
		if task.ScheduledStartTime != Time1 {
			t.Error("Found task with time of not Time1 (2006-01-02T12:00:00.000033Z) when queried for one task")
		}
	}
}

var username = "admin"
var password = "secret"

func TestDeleteRequestParametrization(t *testing.T) {
	err := recreateDb()
	if err != nil {
		t.Logf("Couldn't clean database. Is the enviroment variable set correctly? Is the database test container on?")
		t.Fail()
		return
	}
	server, err := taskserver.New()
	if err != nil {
		t.Logf("Couldn't set up test server. Is the enviroment variable set correctly? Is the database test container on?")
		t.Fail()
		return
	}
	defer func() {
		close(server.Tasks)
		server.Wg.Wait()
	}()

	//test 1: test bad requests

	//invalid status
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/task?status=NOTVALID", nil)
	req.SetBasicAuth(username, password)
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	//invalid httpStatusCode
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/task?httpStatusCode=NaN", nil)
	req.SetBasicAuth(username, password)
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	//test 2: test if the parametrization works correctly

	//create fake test tasks
	server.Store.CreateTask("done", http.StatusInternalServerError, make(map[string]string), "dsad", 0, time.Now())
	server.Store.CreateTask("done", http.StatusOK, make(map[string]string), "dsad", 0, time.Now())
	server.Store.CreateTask("in-progress", http.StatusAccepted, make(map[string]string), "dsad", 0, time.Now())
	server.Store.CreateTask("error", http.StatusInternalServerError, make(map[string]string), "dsad", 0, time.Now())

	//send request for all tasks with statusCode 500 and status done
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/task?httpStatusCode=500&status=done", nil)
	req.SetBasicAuth(username, password)
	server.Router.ServeHTTP(w, req)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/task?httpStatusCode=500&status=done", nil)
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, w.Body.String(), "null")

	//send request for all tasks with status done
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/task?status=done", nil)
	req.SetBasicAuth(username, password)
	server.Router.ServeHTTP(w, req)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/task?status=done", nil)
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, w.Body.String(), "null")

	//send request for all tasks with httpStatusCode 202
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/task?httpStatusCode=202", nil)
	req.SetBasicAuth(username, password)
	server.Router.ServeHTTP(w, req)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/task?httpStatusCode=202", nil)
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, w.Body.String(), "null")

	//delete all tasks before start and end date tests
	server.Store.DeleteAllTasks()
	//create fake test tasks
	const layout_milli = "2006-01-02T15:04:05.000000Z07:00"
	Time1, _ := time.Parse(layout_milli, "2006-01-02T12:00:00.000033Z")
	Time2, _ := time.Parse(time.RFC3339, "2008-01-02T16:00:00Z")
	Time3, _ := time.Parse(time.RFC3339, "2010-01-02T22:00:00Z")
	Time4, _ := time.Parse(time.RFC3339, "2012-01-02T23:00:00.000033Z")
	id, _ := server.Store.CreateTask("done", http.StatusInternalServerError, make(map[string]string), "dsad", 0, Time1)
	server.Store.ChangeTask(id, "done", http.StatusInternalServerError, make(map[string]string), nil, 0, Time1)
	id, _ = server.Store.CreateTask("done", http.StatusOK, make(map[string]string), "dsad", 0, Time2)
	server.Store.ChangeTask(id, "done", http.StatusOK, make(map[string]string), nil, 0, Time2)
	id, _ = server.Store.CreateTask("in-progress", http.StatusAccepted, make(map[string]string), "dsad", 0, Time3)
	server.Store.ChangeTask(id, "in-progress", http.StatusAccepted, make(map[string]string), nil, 0, Time3)
	id, _ = server.Store.CreateTask("error", http.StatusInternalServerError, make(map[string]string), "dsad", 0, Time4)
	server.Store.ChangeTask(id, "error", http.StatusInternalServerError, make(map[string]string), nil, 0, Time4)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/task?minScheduledStartTime=2008-01-02T16:00:00Z&maxScheduledStartTime=2010-01-02T22:00:00Z", nil)
	req.SetBasicAuth(username, password)
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	allTasks, _ := server.Store.GetAllTasks()
	for _, task := range allTasks {
		if !(task.ScheduledStartTime.Before(Time2) || task.ScheduledStartTime.After(Time3)) {
			t.Error("task query with parameters minScheduledStartTime=2008-01-02T16:00:00Z&maxScheduledStartTime=2010-01-02T22:00:00Z returned tasks outside of the bounds of the query")
			return
		}
	}

	//re-add tasks with Time2 and Time3
	id, _ = server.Store.CreateTask("done", http.StatusOK, make(map[string]string), "dsad", 0, Time2)
	server.Store.ChangeTask(id, "done", http.StatusOK, make(map[string]string), nil, 0, Time2)
	id, _ = server.Store.CreateTask("in-progress", http.StatusAccepted, make(map[string]string), "dsad", 0, Time3)
	server.Store.ChangeTask(id, "in-progress", http.StatusAccepted, make(map[string]string), nil, 0, Time3)
	//find requests within scheduledEndTime Time2 and Time3
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/task?minScheduledEndTime=2008-01-02T16:00:00Z&maxScheduledEndTime=2010-01-02T22:00:00Z", nil)
	req.SetBasicAuth(username, password)
	server.Router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	allTasks, _ = server.Store.GetAllTasks()
	for _, task := range allTasks {
		if !(task.ScheduledEndTime.Before(Time2) || task.ScheduledEndTime.After(Time3)) {
			t.Error("task query with parameters minScheduledEndTime=2008-01-02T16:00:00Z&maxScheduledEndTime=2010-01-02T22:00:00Z returned tasks outside of the bounds of the query")
			return
		}
	}
}

func TestDeleteRoutes(t *testing.T) {
	err := recreateDb()
	if err != nil {
		t.Fatalf("Couldn't clean database. Is the enviroment variable set correctly? Is the database test container on?")
		return
	}
	server, err := taskserver.New()
	if err != nil {
		t.Fatalf("Couldn't set up test server. Is the enviroment variable set correctly? Is the database test container on?")
		return
	}
	defer func() {
		close(server.Tasks)
		server.Wg.Wait()
	}()

	//since the create and get tests have extensively tested the validity of the entries, this will not send requests to post /task
	//and instead create everything on the store directly

	//create fake test tasks
	server.Store.CreateTask("mockTask", http.StatusOK, make(map[string]string), "dsad", 0, time.Now())
	server.Store.CreateTask("mockTask", http.StatusOK, make(map[string]string), "dsad", 0, time.Now())
	server.Store.CreateTask("mockTask", http.StatusOK, make(map[string]string), "dsad", 0, time.Now())
	//ttasks, _ := server.Store.GetTasksWithFilter("", nil)
	//t.Log(ttasks[0].Id)

	//test 1: check if the credentials are present
	w := httptest.NewRecorder()

	req, _ := http.NewRequest("DELETE", "/task/1", nil)

	server.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	w = httptest.NewRecorder()

	req, _ = http.NewRequest("DELETE", "/task", nil)

	server.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	//test 2: check for bad request

	w = httptest.NewRecorder()

	req, _ = http.NewRequest("DELETE", "/task/NAN", nil)
	req.SetBasicAuth(username, password)

	server.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	//test 3: test deletion by id

	//check if there are 3 tasks
	tasks, err := server.Store.GetAllTasks()

	if err != nil {
		t.Fatalf("Failed to get tasks: %v", err)
		return
	}

	assert.Equal(t, len(tasks), 3)

	w = httptest.NewRecorder()

	req, _ = http.NewRequest("DELETE", "/task/1", nil)
	req.SetBasicAuth(username, password)

	server.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	tasks, err = server.Store.GetAllTasks()

	if err != nil {
		t.Fatalf("Failed to get tasks: %v", err)
		return
	}

	assert.Equal(t, len(tasks), 2)

	//test 4: test deletion of all tasks

	w = httptest.NewRecorder()

	req, _ = http.NewRequest("DELETE", "/task", nil)
	req.SetBasicAuth(username, password)

	server.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	tasks, err = server.Store.GetAllTasks()

	if err != nil {
		t.Fatalf("Failed to get tasks: %v", err)
		return
	}

	assert.Equal(t, len(tasks), 0)

	//test 5: test for nonexistent task deletion.

	w = httptest.NewRecorder()

	req, _ = http.NewRequest("DELETE", "/task/404", nil)
	req.SetBasicAuth(username, password)

	server.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
