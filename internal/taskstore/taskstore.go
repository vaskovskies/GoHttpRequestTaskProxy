package taskstore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
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
	Headers            map[string]string `json:"headers"`            // Json array containing the headers of the HTTP response (optional)
	Body               string            `json:"body"`               // The body of the HTTP response
	Length             int64             `json:"length"`             // The content length of the HTTP response
	ScheduledStartTime time.Time         `json:"scheduledStartTime"` // The time and date at which the task was sent to the server for processing
	ScheduledEndTime   *time.Time        `json:"scheduledEndTime"`   // The time and date at which the task was done processing
}

type TaskStore struct {
	dbPool *pgxpool.Pool
	sync.Mutex
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
    headers JSONB NOT NULL,
    body TEXT NOT NULL,
    length BIGINT NOT NULL,
    scheduled_start_time TIMESTAMP NOT NULL,
    scheduled_end_time TIMESTAMP NULL
);
`)
	return err
}

// CreateTask creates a new task in the store.

func (ts *TaskStore) CreateTask(status string, httpStatusCode int, headers map[string]string, length int64, scheduledStartTime time.Time) (id int64, err error) {
	ts.Lock()
	defer ts.Unlock()

	err = ts.dbPool.QueryRow(context.Background(),
		`INSERT INTO tasks (status,http_status_code,headers,body,length,scheduled_start_time,scheduled_end_time) 
		VALUES ($1,$2,$3,'',$4,$5,NULL) 
		RETURNING id`,
		status, httpStatusCode, headers, length, scheduledStartTime).Scan(&id)
	if err != nil {
		return -1, err
	}
	return id, nil
}

// GetTask retrieves a task from the store, by id. If no such id exists, an
// error is returned.
func (ts *TaskStore) GetTask(id int64) (task Task, err error) {

	ts.Lock()
	defer ts.Unlock()

	var headersJSON []byte
	err = ts.dbPool.QueryRow(context.Background(),
		"SELECT id,status,http_status_code,headers,body,length,scheduled_start_time,scheduled_end_time FROM tasks where id=$1",
		id).Scan(&task.Id, &task.Status, &task.HttpStatusCode, &headersJSON, &task.Body, &task.Length, &task.ScheduledStartTime, &task.ScheduledEndTime)
	if err != nil {
		println(err.Error())
		return Task{}, err
	}
	task.Headers = make(map[string]string)
	if err := json.Unmarshal(headersJSON, &task.Headers); err != nil {
		return Task{}, err
	}
	return task, nil
}

// DeleteTask deletes the task with the given id. If no such id exists, an error
// is returned.
func (ts *TaskStore) DeleteTask(id int64) error {
	ts.Lock()
	defer ts.Unlock()

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

// DeleteAllTasks deletes all tasks in the store.
func (ts *TaskStore) DeleteAllTasks() error {
	ts.Lock()
	defer ts.Unlock()

	_, err := ts.dbPool.Exec(context.Background(), "TRUNCATE TABLE tasks")

	if err != nil {
		return err
	}

	return nil
}

// GetAllTasks returns all the tasks in the store, in arbitrary order.
func (ts *TaskStore) GetAllTasks(status string, httpStatusCode *int) ([]Task, error) {
	ts.Lock()
	defer ts.Unlock()

	//build sql string
	queryBuilder := squirrel.Select("id,status,http_status_code,headers,body,length,scheduled_start_time,scheduled_end_time").From("tasks")
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
		var headersJSON []byte

		if err := rows.Scan(&task.Id, &task.Status, &task.HttpStatusCode, &headersJSON, &task.Body, &task.Length, &task.ScheduledStartTime, &task.ScheduledEndTime); err != nil {
			return nil, err
		}
		task.Headers = make(map[string]string)
		if err := json.Unmarshal(headersJSON, &task.Headers); err != nil {
			return nil, err
		}
		allTasks = append(allTasks, task)
	}

	return allTasks, nil
}

func (ts *TaskStore) ChangeTask(id int64, status string, httpStatusCode int, headers map[string]string, body string, length int64, scheduledStartTime time.Time, scheduledEndTime time.Time) error {
	ts.Lock()
	defer ts.Unlock()
	_, err := ts.dbPool.Exec(context.Background(),
		`UPDATE TASKS 
		SET status=$2,http_status_code=$3,headers=$4,body=$5,length=$6,scheduled_start_time=$7,scheduled_end_time=$8 
		WHERE id=$1;`,
		id, status, httpStatusCode, headers, body, length, scheduledStartTime, scheduledEndTime)
	if err != nil {
		return err
	}
	return nil
}
