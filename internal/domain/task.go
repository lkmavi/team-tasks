package domain

import (
	"time"

	"github.com/google/uuid"
)

// TaskStatus represents the lifecycle state of a task.
type TaskStatus string

const (
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
)

// TaskPriority represents the urgency level of a task.
type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "medium"
	PriorityHigh   TaskPriority = "high"
)

// Task is the central domain object representing a unit of work within a team.
type Task struct {
	ID          uuid.UUID
	TeamID      uuid.UUID
	CreatedBy   uuid.UUID
	AssigneeID  *uuid.UUID
	Title       string
	Description *string
	Status      TaskStatus
	Priority    TaskPriority
	DueDate     *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CreateTaskInput carries the fields supplied by the caller when creating a new task.
type CreateTaskInput struct {
	TeamID      uuid.UUID
	Title       string
	Description *string
	AssigneeID  *uuid.UUID
	DueDate     *time.Time
	Priority    TaskPriority
}

// UpdateTaskInput carries the optional fields that may be changed on an existing task.
// A nil pointer means "no change".
type UpdateTaskInput struct {
	Title       *string
	Description *string
	Status      *TaskStatus
	AssigneeID  *uuid.UUID
	DueDate     *time.Time
	Priority    *TaskPriority
}

// TaskFilter contains the optional criteria for listing tasks with pagination.
type TaskFilter struct {
	TeamID     *uuid.UUID
	Status     *TaskStatus
	AssigneeID *uuid.UUID
	Page       int
	Limit      int
}

// TaskHistory records a single field change on a task for audit purposes.
type TaskHistory struct {
	ID        int64
	TaskID    uuid.UUID
	ChangedBy uuid.UUID
	Field     string
	OldValue  *string
	NewValue  *string
	ChangedAt time.Time
}
