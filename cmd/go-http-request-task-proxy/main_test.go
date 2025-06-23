package main

import (
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
	CREATE TABLE tasks (
    id BIGSERIAL PRIMARY KEY, 
    status VARCHAR(50) NOT NULL,
    http_status_code INT NOT NULL,
    headers JSONB NOT NULL,
    body TEXT NOT NULL,
    length BIGINT NOT NULL,
    scheduled_start_time TIMESTAMP NOT NULL,
    scheduled_end_time TIMESTAMP NULL
);`)

	return nil
}

func TestCreateRoute(t *testing.T) {

	err := recreateDb()
	if err != nil {
		t.Logf("Couldn't clean database. Is the enviroment variable set correctly? Is the database test container on?")
		t.Fail()
		return
	}
	server, err := setupServer()
	if err != nil {
		t.Logf("Couldn't set up test server. Is the enviroment variable set correctly? Is the database test container on?")
		t.Fail()
		return
	}

	w := httptest.NewRecorder()
	task := RequestBody{
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

	req, _ := http.NewRequest("POST", "/task/", bytes.NewBuffer(taskJSON))
	server.router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, `{"Id":1}`, w.Body.String())

	//test 2: check for bad request
	w = httptest.NewRecorder()

	req, _ = http.NewRequest("POST", "/task/", nil)
	server.router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
}

func TestGetRoutes(t *testing.T) {
	err := recreateDb()
	if err != nil {
		t.Logf("Couldn't clean database. Is the enviroment variable set correctly? Is the database test container on?")
		t.Fail()
		return
	}
	server, err := setupServer()
	if err != nil {
		t.Logf("Couldn't set up test server. Is the enviroment variable set correctly? Is the database test container on?")
		t.Fail()
		return
	}

	//send requests get responses. One is a correct request, the other one fails

	w := httptest.NewRecorder()

	task := RequestBody{
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

	req, _ := http.NewRequest("POST", "/task/", bytes.NewBuffer(taskJSON))
	server.router.ServeHTTP(w, req)

	w = httptest.NewRecorder()
	task = RequestBody{
		Method: "POST",
		Url:    "/task/",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: `{"url": "http://invalid-url"}`,
	}

	req, _ = http.NewRequest("POST", "/task/", bytes.NewBuffer([]byte(task.Body)))
	server.router.ServeHTTP(w, req)

	//test 1: get by id
	w = httptest.NewRecorder()

	req, _ = http.NewRequest("GET", "/task/1", nil)

	server.router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var taskResponse taskstore.Task
	err = json.Unmarshal(w.Body.Bytes(), &taskResponse)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
		return
	}

	assertIdIsOneTaskResponse := func(task taskstore.Task) {
		//check if id is correct
		assert.Equal(t, task.Id, int64(1))
		//check if the response is expected
		assert.Equal(t, task.ResponseBody, `{
  "userId": 1,
  "id": 1,
  "title": "delectus aut autem",
  "completed": false
}`)
		//check if the headers are there
		if len(task.ResponseHeaders) == 0 {
			t.Error("Headers map is empty when it shouldn't be")
		}
		//check if http status code is correct
		assert.Equal(t, task.HttpStatusCode, 200)
		//check if the status of the task is done
		assert.Equal(t, task.Status, "done")
	}
	assertIdIsOneTaskResponse(taskResponse)

	t.Log("Get by id endpoint works correctly")

	//test 2: get all tasks
	w = httptest.NewRecorder()

	req, _ = http.NewRequest("GET", "/task/", nil)

	server.router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var allTasksResponse []taskstore.Task
	err = json.Unmarshal(w.Body.Bytes(), &allTasksResponse)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
		return
	}
	assertIdIsOneTaskResponse(allTasksResponse[0])

	//check if id is correct
	assert.Equal(t, allTasksResponse[1].Id, int64(2))
	//check if the response is expected
	assert.Equal(t, allTasksResponse[1].ResponseBody, `Get "http://invalid-url": dial tcp: lookup invalid-url: no such host`)
	//check if the headers are not present
	if len(allTasksResponse[1].ResponseHeaders) != 0 {
		t.Error("Headers map is empty as it should be")
	}
	//check if http status code is correct
	assert.Equal(t, allTasksResponse[1].HttpStatusCode, 500)
	//check if the status of the taskResponse[1] is done
	assert.Equal(t, allTasksResponse[1].Status, "error")

	//assert.Equal(t, `{"Id":1}`, w.Body.String())
	t.Log("Get all taks endpoint works correctly")

	//test 3: check get on nonexistent tasks
	w = httptest.NewRecorder()

	req, _ = http.NewRequest("GET", "/task/404", nil)

	server.router.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)

	//test 4: test incorrect id format
	w = httptest.NewRecorder()

	req, _ = http.NewRequest("GET", "/task/NAN", nil)

	server.router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
}

func TestGetRequestParametrization(t *testing.T) {
	err := recreateDb()
	if err != nil {
		t.Logf("Couldn't clean database. Is the enviroment variable set correctly? Is the database test container on?")
		t.Fail()
		return
	}
	server, err := setupServer()
	if err != nil {
		t.Logf("Couldn't set up test server. Is the enviroment variable set correctly? Is the database test container on?")
		t.Fail()
		return
	}

	//test 1: test bad requests

	//invalid status
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/task?status=NOTVALID", nil)
	server.router.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
	//invalid httpStatusCode
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/task?httpStatusCode=NaN", nil)
	server.router.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)

	//test 2: test if the parametrization works correctly

	//create fake test tasks
	server.store.CreateTask("done", 500, make(map[string]string), "dsad", 0, time.Now())
	server.store.CreateTask("done", 200, make(map[string]string), "dsad", 0, time.Now())
	server.store.CreateTask("in-progress", 202, make(map[string]string), "dsad", 0, time.Now())
	server.store.CreateTask("error", 500, make(map[string]string), "dsad", 0, time.Now())

	//send request for all tasks with statusCode 500
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/task?httpStatusCode=500", nil)
	server.router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
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
	server.router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
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
	server.router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
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
}

func TestDeleteRequestParametrization(t *testing.T) {
	err := recreateDb()
	if err != nil {
		t.Logf("Couldn't clean database. Is the enviroment variable set correctly? Is the database test container on?")
		t.Fail()
		return
	}
	server, err := setupServer()
	if err != nil {
		t.Logf("Couldn't set up test server. Is the enviroment variable set correctly? Is the database test container on?")
		t.Fail()
		return
	}

	//test 1: test bad requests

	//invalid status
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/task?status=NOTVALID", nil)
	req.SetBasicAuth(username, password)
	server.router.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
	//invalid httpStatusCode
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/task?httpStatusCode=NaN", nil)
	req.SetBasicAuth(username, password)
	server.router.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)

	//test 2: test if the parametrization works correctly

	//create fake test tasks
	server.store.CreateTask("done", 500, make(map[string]string), "dsad", 0, time.Now())
	server.store.CreateTask("done", 200, make(map[string]string), "dsad", 0, time.Now())
	server.store.CreateTask("in-progress", 202, make(map[string]string), "dsad", 0, time.Now())
	server.store.CreateTask("error", 500, make(map[string]string), "dsad", 0, time.Now())

	//send request for all tasks with statusCode 500 and status done
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/task?httpStatusCode=500&status=done", nil)
	req.SetBasicAuth(username, password)
	server.router.ServeHTTP(w, req)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/task?httpStatusCode=500&status=done", nil)
	server.router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, w.Body.String(), "null")

	//send request for all tasks with status done
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/task?status=done", nil)
	req.SetBasicAuth(username, password)
	server.router.ServeHTTP(w, req)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/task?status=done", nil)
	server.router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, w.Body.String(), "null")

	//send request for all tasks with httpStatusCode 202
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/task?httpStatusCode=202", nil)
	req.SetBasicAuth(username, password)
	server.router.ServeHTTP(w, req)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/task?httpStatusCode=202", nil)
	server.router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, w.Body.String(), "null")
}

var username = "admin"
var password = "secret"

func TestDeleteRoutes(t *testing.T) {
	err := recreateDb()
	if err != nil {
		t.Logf("Couldn't clean database. Is the enviroment variable set correctly? Is the database test container on?")
		t.Fail()
		return
	}
	server, err := setupServer()
	if err != nil {
		t.Logf("Couldn't set up test server. Is the enviroment variable set correctly? Is the database test container on?")
		t.Fail()
		return
	}

	//since the create and get tests have extensively tested the validity of the entries, this will not send requests to post /task/
	//and instead create everything on the store directly

	//create fake test tasks
	server.store.CreateTask("mockTask", 200, make(map[string]string), "dsad", 0, time.Now())
	server.store.CreateTask("mockTask", 200, make(map[string]string), "dsad", 0, time.Now())
	server.store.CreateTask("mockTask", 200, make(map[string]string), "dsad", 0, time.Now())
	ttasks, _ := server.store.GetAllTasks("", nil)
	t.Log(ttasks[0].Id)

	//test 1: check if the credentials are present
	w := httptest.NewRecorder()

	req, _ := http.NewRequest("DELETE", "/task/1", nil)

	server.router.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)

	w = httptest.NewRecorder()

	req, _ = http.NewRequest("DELETE", "/task/", nil)

	server.router.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)

	//test 2: check for bad request

	w = httptest.NewRecorder()

	req, _ = http.NewRequest("DELETE", "/task/NAN", nil)
	req.SetBasicAuth(username, password)

	server.router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)

	//test 3: test deletion by id

	//check if there are 3 tasks
	tasks, err := server.store.GetAllTasks("", nil)

	if err != nil {
		t.Fatalf("Failed to get tasks: %v", err)
		return
	}

	assert.Equal(t, len(tasks), 3)

	w = httptest.NewRecorder()

	req, _ = http.NewRequest("DELETE", "/task/1", nil)
	req.SetBasicAuth(username, password)

	server.router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	tasks, err = server.store.GetAllTasks("", nil)

	if err != nil {
		t.Fatalf("Failed to get tasks: %v", err)
		return
	}

	assert.Equal(t, len(tasks), 2)

	//test 4: test deletion of all tasks

	w = httptest.NewRecorder()

	req, _ = http.NewRequest("DELETE", "/task/", nil)
	req.SetBasicAuth(username, password)

	server.router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	tasks, err = server.store.GetAllTasks("", nil)

	if err != nil {
		t.Fatalf("Failed to get tasks: %v", err)
		return
	}

	assert.Equal(t, len(tasks), 0)

	//test 5: test for nonexistent task deletion.

	w = httptest.NewRecorder()

	req, _ = http.NewRequest("DELETE", "/task/404", nil)
	req.SetBasicAuth(username, password)

	server.router.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}
