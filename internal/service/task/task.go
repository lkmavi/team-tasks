package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/lkmavi/team-tasks/internal/domain"
	"github.com/lkmavi/team-tasks/pkg/slogx"
)

// TaskSaver persists a new task.
type TaskSaver interface {
	SaveTask(ctx context.Context, task domain.Task) error
}

// TaskGetter retrieves tasks from storage.
type TaskGetter interface {
	GetTaskByID(ctx context.Context, id uuid.UUID) (domain.Task, error)
	ListTasks(ctx context.Context, filter domain.TaskFilter) ([]domain.Task, int, error)
}

// TaskUpdater updates an existing task and its history atomically.
type TaskUpdater interface {
	UpdateTaskWithHistory(ctx context.Context, task domain.Task, entries []domain.TaskHistory) error
}

// HistoryStore reads task change history.
type HistoryStore interface {
	ListHistory(ctx context.Context, taskID uuid.UUID) ([]domain.TaskHistory, error)
}

// MemberChecker verifies team membership.
type MemberChecker interface {
	GetMemberRole(ctx context.Context, teamID, userID uuid.UUID) (domain.Role, error)
}

// Cache is a short-lived key-value store for task list results.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, data []byte, ttl time.Duration) error
	Invalidate(ctx context.Context, pattern string) error
}

// Service handles task lifecycle and change tracking.
type Service struct {
	taskSaver   TaskSaver
	taskGetter  TaskGetter
	taskUpdater TaskUpdater
	history     HistoryStore
	members     MemberChecker
	cache       Cache
	ttl         time.Duration
}

// New creates a task Service with the given storage adapters, member checker, and cache.
func New(
	taskSaver TaskSaver,
	taskGetter TaskGetter,
	taskUpdater TaskUpdater,
	history HistoryStore,
	members MemberChecker,
	cache Cache,
	ttl time.Duration,
) *Service {
	return &Service{
		taskSaver:   taskSaver,
		taskGetter:  taskGetter,
		taskUpdater: taskUpdater,
		history:     history,
		members:     members,
		cache:       cache,
		ttl:         ttl,
	}
}

func (s *Service) Create(ctx context.Context, creatorID uuid.UUID, input domain.CreateTaskInput) (domain.Task, error) {
	const op = "task.Service.Create"

	log := slogx.FromContext(ctx).With(
		slog.String("op", op),
		slog.String("creator_id", creatorID.String()),
		slog.String("team_id", input.TeamID.String()),
	)

	log.Info("creating task")

	_, err := s.members.GetMemberRole(ctx, input.TeamID, creatorID)
	if errors.Is(err, domain.ErrNotFound) {
		log.Warn("creator is not a team member")
		return domain.Task{}, domain.ErrForbidden
	}
	if err != nil {
		log.Error("failed to check membership", slogx.Err(err))
		return domain.Task{}, fmt.Errorf("%s: get member role: %w", op, err)
	}

	if input.AssigneeID != nil {
		if _, err := s.members.GetMemberRole(ctx, input.TeamID, *input.AssigneeID); errors.Is(err, domain.ErrNotFound) {
			log.Warn("assignee is not a team member")
			return domain.Task{}, fmt.Errorf("%w: assignee is not a team member", domain.ErrForbidden)
		} else if err != nil {
			log.Error("failed to check assignee membership", slogx.Err(err))
			return domain.Task{}, fmt.Errorf("%s: check assignee membership: %w", op, err)
		}
	}

	priority := domain.PriorityMedium
	if input.Priority != "" {
		priority = input.Priority
	}

	id, err := uuid.NewV7()
	if err != nil {
		return domain.Task{}, fmt.Errorf("%s: generate task id: %w", op, err)
	}

	now := time.Now().UTC()
	task := domain.Task{
		ID:          id,
		TeamID:      input.TeamID,
		CreatedBy:   creatorID,
		AssigneeID:  input.AssigneeID,
		Title:       input.Title,
		Description: input.Description,
		Status:      domain.StatusTodo,
		Priority:    priority,
		DueDate:     input.DueDate,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err = s.taskSaver.SaveTask(ctx, task); err != nil {
		log.Error("failed to save task", slogx.Err(err))
		return domain.Task{}, fmt.Errorf("%s: save task: %w", op, err)
	}

	if err = s.cache.Invalidate(ctx, cachePattern(input.TeamID)); err != nil {
		log.Warn("failed to invalidate cache", slogx.Err(err))
	}

	log.Info("task created successfully", slog.String("task_id", task.ID.String()))
	return task, nil
}

func (s *Service) List(ctx context.Context, callerID uuid.UUID, filter domain.TaskFilter) ([]domain.Task, int, error) {
	const op = "task.Service.List"

	log := slogx.FromContext(ctx).With(slog.String("op", op))

	log.Info("listing tasks")

	if filter.TeamID == nil {
		return nil, 0, domain.ErrForbidden
	}

	if _, err := s.members.GetMemberRole(ctx, *filter.TeamID, callerID); errors.Is(err, domain.ErrNotFound) {
		log.Warn("caller is not a team member")
		return nil, 0, domain.ErrForbidden
	} else if err != nil {
		log.Error("failed to check membership", slogx.Err(err))
		return nil, 0, fmt.Errorf("%s: get member role: %w", op, err)
	}

	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	key := cacheKey(filter)
	if data, err := s.cache.Get(ctx, key); err == nil {
		var cached cachedList
		if err = json.Unmarshal(data, &cached); err == nil {
			log.Info("cache hit", slog.Int("total", cached.Total))
			return cached.Tasks, cached.Total, nil
		}
	}

	tasks, total, err := s.taskGetter.ListTasks(ctx, filter)
	if err != nil {
		log.Error("failed to list tasks", slogx.Err(err))
		return nil, 0, fmt.Errorf("%s: list tasks: %w", op, err)
	}

	if data, err := json.Marshal(cachedList{Tasks: tasks, Total: total}); err == nil {
		if err = s.cache.Set(ctx, key, data, s.ttl); err != nil {
			log.Warn("failed to populate cache", slogx.Err(err))
		}
	}

	log.Info("tasks listed", slog.Int("total", total))
	return tasks, total, nil
}

func (s *Service) Update(ctx context.Context, callerID, taskID uuid.UUID, input domain.UpdateTaskInput) (domain.Task, error) {
	const op = "task.Service.Update"

	log := slogx.FromContext(ctx).With(
		slog.String("op", op),
		slog.String("task_id", taskID.String()),
		slog.String("caller_id", callerID.String()),
	)

	log.Info("updating task")

	task, err := s.taskGetter.GetTaskByID(ctx, taskID)
	if err != nil {
		log.Error("failed to get task", slogx.Err(err))
		return domain.Task{}, fmt.Errorf("%s: get task: %w", op, err)
	}

	_, err = s.members.GetMemberRole(ctx, task.TeamID, callerID)
	if errors.Is(err, domain.ErrNotFound) {
		log.Warn("caller is not a team member")
		return domain.Task{}, domain.ErrForbidden
	}
	if err != nil {
		log.Error("failed to check membership", slogx.Err(err))
		return domain.Task{}, fmt.Errorf("%s: get member role: %w", op, err)
	}

	if input.AssigneeID != nil && !uuidPtrEqual(task.AssigneeID, input.AssigneeID) {
		if _, err := s.members.GetMemberRole(ctx, task.TeamID, *input.AssigneeID); errors.Is(err, domain.ErrNotFound) {
			log.Warn("assignee is not a team member")
			return domain.Task{}, fmt.Errorf("%w: assignee is not a team member", domain.ErrForbidden)
		} else if err != nil {
			log.Error("failed to check assignee membership", slogx.Err(err))
			return domain.Task{}, fmt.Errorf("%s: check assignee membership: %w", op, err)
		}
	}

	entries := buildHistoryEntries(&task, input, taskID, callerID)
	task.UpdatedAt = time.Now().UTC()
	for i := range entries {
		entries[i].ChangedAt = task.UpdatedAt
	}

	if err = s.taskUpdater.UpdateTaskWithHistory(ctx, task, entries); err != nil {
		log.Error("failed to update task", slogx.Err(err))
		return domain.Task{}, fmt.Errorf("%s: update task: %w", op, err)
	}

	if err = s.cache.Invalidate(ctx, cachePattern(task.TeamID)); err != nil {
		log.Warn("failed to invalidate cache", slogx.Err(err))
	}

	log.Info("task updated successfully", slog.Int("history_entries", len(entries)))
	return task, nil
}

func buildHistoryEntries(task *domain.Task, input domain.UpdateTaskInput, taskID, callerID uuid.UUID) []domain.TaskHistory {
	var entries []domain.TaskHistory

	if input.Title != nil && *input.Title != task.Title {
		old := task.Title
		entries = append(entries, domain.TaskHistory{TaskID: taskID, ChangedBy: callerID, Field: "title", OldValue: &old, NewValue: input.Title})
		task.Title = *input.Title
	}
	if input.Description != nil && !strPtrEqual(task.Description, input.Description) {
		entries = append(entries, domain.TaskHistory{TaskID: taskID, ChangedBy: callerID, Field: "description", OldValue: task.Description, NewValue: input.Description})
		task.Description = input.Description
	}
	if input.Status != nil && *input.Status != task.Status {
		old, nw := string(task.Status), string(*input.Status)
		entries = append(entries, domain.TaskHistory{TaskID: taskID, ChangedBy: callerID, Field: "status", OldValue: &old, NewValue: &nw})
		task.Status = *input.Status
	}
	if input.Priority != nil && *input.Priority != task.Priority {
		old, nw := string(task.Priority), string(*input.Priority)
		entries = append(entries, domain.TaskHistory{TaskID: taskID, ChangedBy: callerID, Field: "priority", OldValue: &old, NewValue: &nw})
		task.Priority = *input.Priority
	}
	if input.AssigneeID != nil && !uuidPtrEqual(task.AssigneeID, input.AssigneeID) {
		old := ""
		if task.AssigneeID != nil {
			old = task.AssigneeID.String()
		}
		nw := input.AssigneeID.String()
		entries = append(entries, domain.TaskHistory{TaskID: taskID, ChangedBy: callerID, Field: "assignee_id", OldValue: &old, NewValue: &nw})
		task.AssigneeID = input.AssigneeID
	}
	if input.DueDate != nil && !timePtrEqual(task.DueDate, input.DueDate) {
		old := formatDate(task.DueDate)
		nw := formatDate(input.DueDate)
		entries = append(entries, domain.TaskHistory{TaskID: taskID, ChangedBy: callerID, Field: "due_date", OldValue: &old, NewValue: &nw})
		task.DueDate = input.DueDate
	}

	return entries
}

func (s *Service) History(ctx context.Context, callerID, taskID uuid.UUID) ([]domain.TaskHistory, error) {
	const op = "task.Service.History"

	log := slogx.FromContext(ctx).With(
		slog.String("op", op),
		slog.String("task_id", taskID.String()),
		slog.String("caller_id", callerID.String()),
	)

	log.Info("fetching task history")

	task, err := s.taskGetter.GetTaskByID(ctx, taskID)
	if err != nil {
		log.Error("failed to get task", slogx.Err(err))
		return nil, fmt.Errorf("%s: get task: %w", op, err)
	}

	_, err = s.members.GetMemberRole(ctx, task.TeamID, callerID)
	if errors.Is(err, domain.ErrNotFound) {
		log.Warn("caller is not a team member")
		return nil, domain.ErrForbidden
	}
	if err != nil {
		log.Error("failed to check membership", slogx.Err(err))
		return nil, fmt.Errorf("%s: get member role: %w", op, err)
	}

	history, err := s.history.ListHistory(ctx, taskID)
	if err != nil {
		log.Error("failed to list history", slogx.Err(err))
		return nil, fmt.Errorf("%s: list history: %w", op, err)
	}

	log.Info("history fetched", slog.Int("entries", len(history)))
	return history, nil
}

type cachedList struct {
	Tasks []domain.Task `json:"tasks"`
	Total int           `json:"total"`
}

const cacheKeyAll = "all"

func cacheKey(f domain.TaskFilter) string {
	teamID := cacheKeyAll
	if f.TeamID != nil {
		teamID = f.TeamID.String()
	}
	status := cacheKeyAll
	if f.Status != nil {
		status = string(*f.Status)
	}
	assignee := cacheKeyAll
	if f.AssigneeID != nil {
		assignee = f.AssigneeID.String()
	}
	return fmt.Sprintf("tasks:%s:%s:%s:p%d:l%d", teamID, status, assignee, f.Page, f.Limit)
}

func cachePattern(teamID uuid.UUID) string {
	return "tasks:" + teamID.String() + ":*"
}

func strPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func uuidPtrEqual(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func timePtrEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func formatDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}
