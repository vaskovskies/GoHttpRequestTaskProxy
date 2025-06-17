package taskstore

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Task struct {
	Id                 int64             `json:"id"`
	Status             string            `json:"status"`
	HttpStatusCode     int               `json:"httpStatusCode"`
	Headers            map[string]string `json:"headers"`
	Body               string            `json:"body"`
	Length             int64             `json:"length"`
	ScheduledStartTime time.Time         `json:"scheduledStartTime"`
	ScheduledEndTime   *time.Time        `json:"scheduledEndTime"`
}

type TaskStore struct {
	dbPool *pgxpool.Pool
	sync.Mutex
}

func (ts *TaskStore) CloseDatabasePool() {
	ts.dbPool.Close()
}

// this is the initializer
func New() *TaskStore {
	ts := &TaskStore{}
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	ts.dbPool = pool
	if err != nil {
		return nil
	}
	err = ts.initDb()
	if err != nil {
		return nil
	}
	return ts
}

// Initiliaze database if ran locally or docker initilization script failed
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

	/*
		task := Task{
			Id:                 ts.nextId,
			Status:             status,
			HttpStatusCode:     httpStatusCode,
			Headers:            headers,
			Body:               "",
			Length:             length,
			ScheduledStartTime: scheduledStartTime,
			ScheduledEndTime:   ""}

		ts.tasks[ts.nextId] = task
		ts.nextId++
	*/
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

	var headersJSON []byte // Temporary variable to hold JSONB data
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

	_, err := ts.dbPool.Exec(context.Background(), "DELETE FROM tasks WHERE id = $1", id)

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
func (ts *TaskStore) GetAllTasks() ([]Task, error) {
	ts.Lock()
	defer ts.Unlock()

	/*
		allTasks := make([]Task, 0, len(ts.tasks))
		for _, task := range ts.tasks {
			allTasks = append(allTasks, task)
		}
	*/
	var allTasks []Task
	rows, err := ts.dbPool.Query(context.Background(),
		"SELECT id,status,http_status_code,headers,body,length,scheduled_start_time,scheduled_end_time FROM tasks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var task Task
		var headersJSON []byte // Temporary variable to hold JSONB data
		//it's failing here
		if err := rows.Scan(&task.Id, &task.Status, &task.HttpStatusCode, &headersJSON, &task.Body, &task.Length, &task.ScheduledStartTime, &task.ScheduledEndTime); err != nil {
			return nil, err
		}
		task.Headers = make(map[string]string)
		if err := json.Unmarshal(headersJSON, &task.Headers); err != nil {
			return nil, err
		}
		allTasks = append(allTasks, task)
	}

	/*
		if err := rows.Err(); err != nil {
			return nil, err
		}
	*/

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
