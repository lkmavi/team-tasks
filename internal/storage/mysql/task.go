package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/lkmavi/team-tasks/internal/domain"
	"github.com/lkmavi/team-tasks/pkg/slogx"
)

// TaskStorage implements task and task history persistence backed by MySQL.
type TaskStorage struct {
	db *sqlx.DB
}

// NewTaskStorage creates a TaskStorage using the provided database connection.
func NewTaskStorage(db *sqlx.DB) *TaskStorage {
	return &TaskStorage{db: db}
}

type taskRow struct {
	ID          []byte              `db:"id"`
	TeamID      []byte              `db:"team_id"`
	CreatedBy   []byte              `db:"created_by"`
	AssigneeID  []byte              `db:"assignee_id"`
	Title       string              `db:"title"`
	Description *string             `db:"description"`
	Status      domain.TaskStatus   `db:"status"`
	Priority    domain.TaskPriority `db:"priority"`
	DueDate     *time.Time          `db:"due_date"`
	CreatedAt   time.Time           `db:"created_at"`
	UpdatedAt   time.Time           `db:"updated_at"`
}

type historyRow struct {
	ID        int64     `db:"id"`
	TaskID    []byte    `db:"task_id"`
	ChangedBy []byte    `db:"changed_by"`
	Field     string    `db:"field"`
	OldValue  *string   `db:"old_value"`
	NewValue  *string   `db:"new_value"`
	ChangedAt time.Time `db:"changed_at"`
}

func (s *TaskStorage) SaveTask(ctx context.Context, task domain.Task) error {
	const op = "mysql.TaskStorage.SaveTask"
	log := slogx.FromContext(ctx).With(slog.String("op", op), slog.String("task_id", task.ID.String()))
	log.Info("saving task")

	var assigneeBytes []byte
	if task.AssigneeID != nil {
		assigneeBytes = uuidToBytes(*task.AssigneeID)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tasks
			(id, team_id, created_by, assignee_id, title, description, status, priority, due_date, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuidToBytes(task.ID), uuidToBytes(task.TeamID), uuidToBytes(task.CreatedBy),
		assigneeBytes, task.Title, task.Description,
		string(task.Status), string(task.Priority),
		task.DueDate, task.CreatedAt, task.UpdatedAt,
	)
	if err != nil {
		log.Error("failed to save task", slogx.Err(err))
		return fmt.Errorf("mysql: save task: %w", err)
	}

	log.Info("task saved")
	return nil
}

func (s *TaskStorage) GetTaskByID(ctx context.Context, id uuid.UUID) (domain.Task, error) {
	const op = "mysql.TaskStorage.GetTaskByID"
	log := slogx.FromContext(ctx).With(slog.String("op", op), slog.String("task_id", id.String()))
	log.Info("fetching task by id")

	var row taskRow
	err := s.db.GetContext(ctx, &row, `
		SELECT id, team_id, created_by, assignee_id, title, description, status, priority, due_date, created_at, updated_at
		FROM tasks WHERE id = ?`,
		uuidToBytes(id),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("task not found")
			return domain.Task{}, domain.ErrNotFound
		}
		log.Error("failed to fetch task", slogx.Err(err))
		return domain.Task{}, fmt.Errorf("mysql: get task by id: %w", err)
	}

	t, err := toTask(row)
	if err != nil {
		log.Error("failed to parse task row", slogx.Err(err))
		return domain.Task{}, err
	}

	log.Info("task fetched")
	return t, nil
}

func (s *TaskStorage) ListTasks(ctx context.Context, filter domain.TaskFilter) ([]domain.Task, int, error) {
	const op = "mysql.TaskStorage.ListTasks"
	log := slogx.FromContext(ctx).With(slog.String("op", op), slog.Int("page", filter.Page), slog.Int("limit", filter.Limit))
	log.Info("listing tasks")

	conds, args := buildTaskFilter(filter)

	whereClause := ""
	if len(conds) > 0 {
		whereClause = "WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := s.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM tasks "+whereClause, args...); err != nil {
		log.Error("failed to count tasks", slogx.Err(err))
		return nil, 0, fmt.Errorf("mysql: count tasks: %w", err)
	}

	offset := (filter.Page - 1) * filter.Limit
	args = append(args, filter.Limit, offset)

	var rows []taskRow
	err := s.db.SelectContext(ctx, &rows, `
		SELECT id, team_id, created_by, assignee_id, title, description, status, priority, due_date, created_at, updated_at
		FROM tasks `+whereClause+` ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		log.Error("failed to list tasks", slogx.Err(err))
		return nil, 0, fmt.Errorf("mysql: list tasks: %w", err)
	}

	tasks := make([]domain.Task, 0, len(rows))
	for i := range rows {
		t, err := toTask(rows[i])
		if err != nil {
			log.Error("failed to parse task row", slogx.Err(err))
			return nil, 0, err
		}
		tasks = append(tasks, t)
	}

	log.Info("tasks listed", slog.Int("total", total), slog.Int("returned", len(tasks)))
	return tasks, total, nil
}

func (s *TaskStorage) UpdateTaskWithHistory(ctx context.Context, task domain.Task, entries []domain.TaskHistory) error {
	const op = "mysql.TaskStorage.UpdateTaskWithHistory"
	log := slogx.FromContext(ctx).With(slog.String("op", op), slog.String("task_id", task.ID.String()))
	log.Info("updating task with history")

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var assigneeBytes []byte
	if task.AssigneeID != nil {
		assigneeBytes = uuidToBytes(*task.AssigneeID)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE tasks SET
			assignee_id = ?, title = ?, description = ?, status = ?, priority = ?, due_date = ?, updated_at = ?
		WHERE id = ?`,
		assigneeBytes, task.Title, task.Description,
		string(task.Status), string(task.Priority),
		task.DueDate, task.UpdatedAt,
		uuidToBytes(task.ID),
	)
	if err != nil {
		log.Error("failed to update task", slogx.Err(err))
		return fmt.Errorf("mysql: update task: %w", err)
	}

	for _, h := range entries {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO task_history (task_id, changed_by, field, old_value, new_value, changed_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			uuidToBytes(h.TaskID), uuidToBytes(h.ChangedBy),
			h.Field, h.OldValue, h.NewValue, h.ChangedAt,
		)
		if err != nil {
			log.Error("failed to insert history entry", slog.String("field", h.Field), slogx.Err(err))
			return fmt.Errorf("mysql: save history: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit: %w", err)
	}

	log.Info("task updated with history", slog.Int("history_entries", len(entries)))
	return nil
}

func (s *TaskStorage) SaveHistory(ctx context.Context, h domain.TaskHistory) error {
	const op = "mysql.TaskStorage.SaveHistory"
	log := slogx.FromContext(ctx).With(
		slog.String("op", op),
		slog.String("task_id", h.TaskID.String()),
		slog.String("field", h.Field),
	)
	log.Info("saving history entry")

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO task_history (task_id, changed_by, field, old_value, new_value, changed_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		uuidToBytes(h.TaskID), uuidToBytes(h.ChangedBy),
		h.Field, h.OldValue, h.NewValue, h.ChangedAt,
	)
	if err != nil {
		log.Error("failed to save history", slogx.Err(err))
		return fmt.Errorf("mysql: save history: %w", err)
	}

	log.Info("history entry saved")
	return nil
}

func (s *TaskStorage) ListHistory(ctx context.Context, taskID uuid.UUID) ([]domain.TaskHistory, error) {
	const op = "mysql.TaskStorage.ListHistory"
	log := slogx.FromContext(ctx).With(slog.String("op", op), slog.String("task_id", taskID.String()))
	log.Info("listing task history")

	var rows []historyRow
	err := s.db.SelectContext(ctx, &rows, `
		SELECT id, task_id, changed_by, field, old_value, new_value, changed_at
		FROM task_history WHERE task_id = ? ORDER BY changed_at ASC`,
		uuidToBytes(taskID),
	)
	if err != nil {
		log.Error("failed to list history", slogx.Err(err))
		return nil, fmt.Errorf("mysql: list history: %w", err)
	}

	entries := make([]domain.TaskHistory, 0, len(rows))
	for _, r := range rows {
		h, err := toHistory(r)
		if err != nil {
			log.Error("failed to parse history row", slogx.Err(err))
			return nil, err
		}
		entries = append(entries, h)
	}

	log.Info("history listed", slog.Int("entries", len(entries)))
	return entries, nil
}

func buildTaskFilter(f domain.TaskFilter) (conds []string, args []any) {
	if f.TeamID != nil {
		conds = append(conds, "team_id = ?")
		args = append(args, uuidToBytes(*f.TeamID))
	}
	if f.Status != nil {
		conds = append(conds, "status = ?")
		args = append(args, string(*f.Status))
	}
	if f.AssigneeID != nil {
		conds = append(conds, "assignee_id = ?")
		args = append(args, uuidToBytes(*f.AssigneeID))
	}
	return
}

func toTask(r taskRow) (domain.Task, error) {
	id, err := bytesToUUID(r.ID)
	if err != nil {
		return domain.Task{}, fmt.Errorf("mysql: parse task id: %w", err)
	}
	teamID, err := bytesToUUID(r.TeamID)
	if err != nil {
		return domain.Task{}, fmt.Errorf("mysql: parse task team_id: %w", err)
	}
	createdBy, err := bytesToUUID(r.CreatedBy)
	if err != nil {
		return domain.Task{}, fmt.Errorf("mysql: parse task created_by: %w", err)
	}
	assigneeID, err := bytesToUUIDPtr(r.AssigneeID)
	if err != nil {
		return domain.Task{}, fmt.Errorf("mysql: parse task assignee_id: %w", err)
	}
	return domain.Task{
		ID:          id,
		TeamID:      teamID,
		CreatedBy:   createdBy,
		AssigneeID:  assigneeID,
		Title:       r.Title,
		Description: r.Description,
		Status:      r.Status,
		Priority:    r.Priority,
		DueDate:     r.DueDate,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}, nil
}

func toHistory(r historyRow) (domain.TaskHistory, error) {
	taskID, err := bytesToUUID(r.TaskID)
	if err != nil {
		return domain.TaskHistory{}, fmt.Errorf("mysql: parse history task_id: %w", err)
	}
	changedBy, err := bytesToUUID(r.ChangedBy)
	if err != nil {
		return domain.TaskHistory{}, fmt.Errorf("mysql: parse history changed_by: %w", err)
	}
	return domain.TaskHistory{
		ID:        r.ID,
		TaskID:    taskID,
		ChangedBy: changedBy,
		Field:     r.Field,
		OldValue:  r.OldValue,
		NewValue:  r.NewValue,
		ChangedAt: r.ChangedAt,
	}, nil
}
