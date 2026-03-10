package agentteam

import (
	"fmt"
	"sync"
)

// TaskQueue manages task queueing and scheduling
type TaskQueue struct {
	mu      sync.RWMutex
	queue   chan *Task
	tasks   map[string]*Task // taskID -> task
	pending int              // number of pending tasks
}

// NewTaskQueue creates a new task queue
func NewTaskQueue(maxSize int) *TaskQueue {
	if maxSize <= 0 {
		maxSize = 100 // Default size
	}

	return &TaskQueue{
		queue:   make(chan *Task, maxSize),
		tasks:   make(map[string]*Task),
		pending: 0,
	}
}

// Enqueue adds a task to the queue
func (q *TaskQueue) Enqueue(task *Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	select {
	case q.queue <- task:
		q.tasks[task.ID] = task
		q.pending++
		return nil
	default:
		return fmt.Errorf("task queue is full")
	}
}

// Dequeue removes and returns the next task
func (q *TaskQueue) Dequeue() (*Task, error) {
	return q.DequeueByID("")
}

// DequeueByID removes a specific task from the queue
func (q *TaskQueue) DequeueByID(taskID string) (*Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, exists := q.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found in queue", taskID)
	}

	// Remove from map
	delete(q.tasks, taskID)
	q.pending--

	return task, nil
}

// Get retrieves a task by ID without dequeuing
func (q *TaskQueue) Get(taskID string) *Task {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.tasks[taskID]
}

// UpdateTask updates the state of a task
func (q *TaskQueue) UpdateTask(taskID string, newState TaskState, result, errorMsg string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	task := q.tasks[taskID]
	if task != nil {
		task.State = newState
		task.Result = result
		task.Error = errorMsg

		if newState == StateCompleted || newState == StateFailed {
			q.pending--
			if q.pending < 0 {
				q.pending = 0
			}
		}
	}
}

// Peek returns the next task without dequeuing
func (q *TaskQueue) Peek() *Task {
	select {
	case task := <-q.queue:
		q.queue <- task // Put it back
		return task
	default:
		return nil
	}
}

// GetPendingCount returns the number of pending tasks
func (q *TaskQueue) GetPendingCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pending
}

// GetCount returns the total number of tasks in the queue
func (q *TaskQueue) GetCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.tasks)
}

// ListTasks returns all tasks in the queue
func (q *TaskQueue) ListTasks() []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	tasks := make([]*Task, 0, len(q.tasks))
	for _, task := range q.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}
