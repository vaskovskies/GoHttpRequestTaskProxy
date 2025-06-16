package taskstore

import (
	"fmt"
	"sync"
)

type Task struct {
	Id                 int64             `json:"id"`
	Status             string            `json:"status"`
	HttpStatusCode     int               `json:"httpStatusCode"`
	Headers            map[string]string `json:"headers"`
	Body               string            `json:"body"`
	Length             int64             `json:"length"`
	ScheduledStartTime string            `json:"scheduledStartTime"`
	ScheduledEndTime   string            `json:"scheduledEndTime"`
}

type TaskStore struct {
	sync.Mutex

	tasks  map[int64]Task
	nextId int64
}

// this is the initializer
func New() *TaskStore {
	ts := &TaskStore{}
	ts.tasks = make(map[int64]Task)
	ts.nextId = 0
	return ts
}

// CreateTask creates a new task in the store.
func (ts *TaskStore) CreateTask(status string, httpStatusCode int, headers map[string]string, length int64, scheduledStartTime string) int64 {
	ts.Lock()
	defer ts.Unlock()

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
	return task.Id
}

// GetTask retrieves a task from the store, by id. If no such id exists, an
// error is returned.
func (ts *TaskStore) GetTask(id int64) (Task, error) {

	ts.Lock()
	defer ts.Unlock()

	t, ok := ts.tasks[id]
	if ok {
		return t, nil
	} else {
		return Task{}, fmt.Errorf("task with id=%d not found", id)
	}
}

// DeleteTask deletes the task with the given id. If no such id exists, an error
// is returned.
func (ts *TaskStore) DeleteTask(id int64) error {
	ts.Lock()
	defer ts.Unlock()

	if _, ok := ts.tasks[id]; !ok {
		return fmt.Errorf("task with id=%d not found", id)
	}

	delete(ts.tasks, id)
	return nil
}

// DeleteAllTasks deletes all tasks in the store.
func (ts *TaskStore) DeleteAllTasks() error {
	ts.Lock()
	defer ts.Unlock()

	ts.tasks = make(map[int64]Task)
	return nil
}

// GetAllTasks returns all the tasks in the store, in arbitrary order.
func (ts *TaskStore) GetAllTasks() []Task {
	ts.Lock()
	defer ts.Unlock()

	allTasks := make([]Task, 0, len(ts.tasks))
	for _, task := range ts.tasks {
		allTasks = append(allTasks, task)
	}
	return allTasks
}

func (ts *TaskStore) ChangeTask(id int64, status string, httpStatusCode int, headers map[string]string, body string, length int64, scheduledStartTime string, scheduledEndTime string) error {
	ts.Lock()
	defer ts.Unlock()
	ts.tasks[id] = Task{
		Id:                 id,
		Status:             status,
		HttpStatusCode:     httpStatusCode,
		Headers:            headers,
		Body:               body,
		Length:             length,
		ScheduledStartTime: scheduledStartTime,
		ScheduledEndTime:   scheduledEndTime,
	}
	return nil
}
