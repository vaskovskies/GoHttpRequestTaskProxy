package taskstore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Task represents a task in the system
// @Description Task model
type Task struct {
	Id                 int64             `json:"id"`                 // The ID of the task
	Status             string            `json:"status"`             // The status of the task:done/in-progress/error
	HttpStatusCode     int               `json:"httpStatusCode"`     // The httpStatusCode of the HTTP response or Internal Server Error (500) in case of server errors
	RequestHeaders     map[string]string `json:"requestHeaders"`     // Json array containing the headers of the HTTP request (optional)
	ResponseHeaders    map[string]string `json:"responseHeaders"`    // Json array containing the headers of the HTTP response
	RequestBody        string            `json:"requestBody"`        // The body of the HTTP request
	ResponseBody       *string           `json:"responseBody"`       // The body of the HTTP response
	Length             int64             `json:"length"`             // The content length of the HTTP response
	ScheduledStartTime time.Time         `json:"scheduledStartTime"` // The time and date at which the task was sent to the server for processing
	ScheduledEndTime   *time.Time        `json:"scheduledEndTime"`   // The time and date at which the task was done processing
}

const (
	StatusDone       = "done"
	StatusInProgress = "in-progress"
	StatusError      = "error"
)

type TaskStore struct {
	dbPool *pgxpool.Pool
	//sync.Mutex
}

func (ts *TaskStore) CloseDatabasePool() {
	ts.dbPool.Close()
}

func New() (*TaskStore, error) {
	ts := &TaskStore{}
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	ts.dbPool = pool
	if err != nil {
		return nil, err
	}
	err = ts.initDb()
	if err != nil {
		return nil, err
	}
	return ts, nil
}

// Initiliaze database if the database is run locally or if docker initilization script failed

func (ts *TaskStore) initDb() error {
	_, err := ts.dbPool.Exec(context.Background(), `
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
);
`)
	return err
}

// CreateTask creates a new task in the store.

func (ts *TaskStore) CreateTask(status string, httpStatusCode int, requestHeaders map[string]string, requestBody string, length int64, scheduledStartTime time.Time) (id int64, err error) {

	err = ts.dbPool.QueryRow(context.Background(),
		`INSERT INTO tasks (status,http_status_code,request_headers,response_headers,request_body,response_body,length,scheduled_start_time,scheduled_end_time) 
		VALUES ($1,$2,$3,NULL,$4,NULL,$5,$6,NULL) 
		RETURNING id`,
		status, httpStatusCode, requestHeaders, requestBody, length, scheduledStartTime).Scan(&id)
	if err != nil {
		return -1, err
	}
	return id, nil
}

// GetTask retrieves a task from the store, by id. If no such id exists, an
// error is returned.
func (ts *TaskStore) GetTask(id int64) (task Task, err error) {

	var requestHeadersJSON []byte
	var responseHeadersJSON []byte
	err = ts.dbPool.QueryRow(context.Background(),
		"SELECT id,status,http_status_code,request_headers,response_headers,request_body,response_body,length,scheduled_start_time,scheduled_end_time FROM tasks where id=$1",
		id).Scan(&task.Id, &task.Status, &task.HttpStatusCode, &requestHeadersJSON, &responseHeadersJSON, &task.RequestBody, &task.ResponseBody, &task.Length, &task.ScheduledStartTime, &task.ScheduledEndTime)
	if err != nil {
		return Task{}, err
	}
	task.RequestHeaders = make(map[string]string)
	if err := json.Unmarshal(requestHeadersJSON, &task.RequestHeaders); err != nil {
		return Task{}, err
	}
	task.ResponseHeaders = make(map[string]string)
	if responseHeadersJSON != nil {
		if err := json.Unmarshal(responseHeadersJSON, &task.ResponseHeaders); err != nil {
			return Task{}, err
		}
	}
	return task, nil
}

// DeleteTask deletes the task with the given id. If no such id exists, an error
// is returned.
func (ts *TaskStore) DeleteTask(id int64) error {

	var exists bool
	//check if task exists
	err := ts.dbPool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM tasks WHERE id = $1)", id).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("task does not exist")
	}

	_, err = ts.dbPool.Exec(context.Background(), "DELETE FROM tasks WHERE id = $1", id)

	if err != nil {
		return err
	}

	return nil
}

// DeleteAllTasks deletes all tasks in the store
func (ts *TaskStore) DeleteAllTasks() error {

	_, err := ts.dbPool.Exec(context.Background(), "TRUNCATE TABLE tasks")

	if err != nil {
		return err
	}

	return nil
}

// DeleteTasksWithFilter deletes all tasks in the store with given status and httpStatusCode.
func (ts *TaskStore) DeleteTasksWithFilter(status string, httpStatusCode *int) error {

	if status == "" && httpStatusCode == nil {
		_, err := ts.dbPool.Exec(context.Background(), "TRUNCATE TABLE tasks")

		if err != nil {
			return err
		}
	} else {
		queryBuilder := squirrel.Delete("tasks")
		if status != "" {
			queryBuilder = queryBuilder.Where("status = ?", status)
		}
		if httpStatusCode != nil {
			queryBuilder = queryBuilder.Where("http_status_code = ?", *httpStatusCode)
		}
		queryBuilder = queryBuilder.PlaceholderFormat(squirrel.Dollar)
		query, args, err := queryBuilder.ToSql()
		if err != nil {
			return err
		}
		ts.dbPool.Exec(context.Background(), query, args...)

	}

	return nil
}

// GetAllTasks returns all the tasks in the store, in arbitrary order.
func (ts *TaskStore) GetAllTasks() ([]Task, error) {
	var allTasks []Task
	rows, err := ts.dbPool.Query(context.Background(),
		"SELECT id,status,http_status_code,request_headers,response_headers,request_body,response_body,length,scheduled_start_time,scheduled_end_time FROM tasks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var task Task

		var requestHeadersJSON []byte
		var responseHeadersJSON []byte

		if err := rows.Scan(&task.Id, &task.Status, &task.HttpStatusCode, &requestHeadersJSON, &responseHeadersJSON, &task.RequestBody, &task.ResponseBody, &task.Length, &task.ScheduledStartTime, &task.ScheduledEndTime); err != nil {
			return nil, err
		}
		task.RequestHeaders = make(map[string]string)
		if err := json.Unmarshal(requestHeadersJSON, &task.RequestHeaders); err != nil {
			return make([]Task, 0), err
		}
		task.ResponseHeaders = make(map[string]string)
		if responseHeadersJSON != nil {
			if err := json.Unmarshal(responseHeadersJSON, &task.ResponseHeaders); err != nil {
				return make([]Task, 0), err
			}
		}
		allTasks = append(allTasks, task)
	}

	return allTasks, nil
}

// GetTasksWithFilter returns tasks with the following status and/or httpStatusCode
func (ts *TaskStore) GetTasksWithFilter(status string, httpStatusCode *int) ([]Task, error) {

	//build sql string
	queryBuilder := squirrel.Select("id,status,http_status_code,request_headers,response_headers,request_body,response_body,length,scheduled_start_time,scheduled_end_time").From("tasks")
	if status != "" {
		queryBuilder = queryBuilder.Where("status = ?", status)
	}
	if httpStatusCode != nil {
		queryBuilder = queryBuilder.Where("http_status_code = ?", *httpStatusCode)
	}
	queryBuilder = queryBuilder.PlaceholderFormat(squirrel.Dollar)
	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, err
	}
	var allTasks []Task
	rows, err := ts.dbPool.Query(context.Background(),
		query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var task Task

		var requestHeadersJSON []byte
		var responseHeadersJSON []byte

		if err := rows.Scan(&task.Id, &task.Status, &task.HttpStatusCode, &requestHeadersJSON, &responseHeadersJSON, &task.RequestBody, &task.ResponseBody, &task.Length, &task.ScheduledStartTime, &task.ScheduledEndTime); err != nil {
			return nil, err
		}
		task.RequestHeaders = make(map[string]string)
		if err := json.Unmarshal(requestHeadersJSON, &task.RequestHeaders); err != nil {
			return make([]Task, 0), err
		}
		task.ResponseHeaders = make(map[string]string)
		if responseHeadersJSON != nil {
			if err := json.Unmarshal(responseHeadersJSON, &task.ResponseHeaders); err != nil {
				return make([]Task, 0), err
			}
		}
		allTasks = append(allTasks, task)
	}

	return allTasks, nil
}

func (ts *TaskStore) ChangeTask(id int64, status string, httpStatusCode int, response_headers map[string]string, responseBody *string, length int64, scheduledEndTime time.Time) error {
	_, err := ts.dbPool.Exec(context.Background(),
		`UPDATE TASKS 
		SET status=$2,http_status_code=$3,response_headers=$4,response_body=$5,length=$6,scheduled_end_time=$7 
		WHERE id=$1;`,
		id, status, httpStatusCode, response_headers, responseBody, length, scheduledEndTime)
	if err != nil {
		return err
	}
	return nil
}
