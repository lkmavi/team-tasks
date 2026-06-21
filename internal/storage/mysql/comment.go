package mysql

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/lkmavi/team-tasks/internal/domain"
	"github.com/lkmavi/team-tasks/pkg/slogx"
)

// CommentStorage implements comment persistence backed by MySQL.
type CommentStorage struct {
	db *sqlx.DB
}

// NewCommentStorage creates a CommentStorage using the provided database connection.
func NewCommentStorage(db *sqlx.DB) *CommentStorage {
	return &CommentStorage{db: db}
}

type commentRow struct {
	ID        int64     `db:"id"`
	TaskID    []byte    `db:"task_id"`
	UserID    []byte    `db:"user_id"`
	Body      string    `db:"body"`
	CreatedAt time.Time `db:"created_at"`
}

func (s *CommentStorage) SaveComment(ctx context.Context, c domain.Comment) (domain.Comment, error) {
	const op = "mysql.CommentStorage.SaveComment"
	log := slogx.FromContext(ctx).With(slog.String("op", op), slog.String("task_id", c.TaskID.String()))
	log.Info("saving comment")

	res, err := s.db.ExecContext(ctx,
		"INSERT INTO task_comments (task_id, user_id, body) VALUES (?, ?, ?)",
		uuidToBytes(c.TaskID), uuidToBytes(c.UserID), c.Body,
	)
	if err != nil {
		log.Error("failed to save comment", slogx.Err(err))
		return domain.Comment{}, fmt.Errorf("mysql: save comment: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return domain.Comment{}, fmt.Errorf("mysql: last insert id: %w", err)
	}

	c.ID = id
	log.Info("comment saved", slog.Int64("comment_id", id))
	return c, nil
}

func (s *CommentStorage) ListComments(ctx context.Context, taskID uuid.UUID) ([]domain.Comment, error) {
	const op = "mysql.CommentStorage.ListComments"
	log := slogx.FromContext(ctx).With(slog.String("op", op), slog.String("task_id", taskID.String()))
	log.Info("listing comments")

	var rows []commentRow
	err := s.db.SelectContext(ctx, &rows,
		"SELECT id, task_id, user_id, body, created_at FROM task_comments WHERE task_id = ? ORDER BY created_at ASC",
		uuidToBytes(taskID),
	)
	if err != nil {
		log.Error("failed to list comments", slogx.Err(err))
		return nil, fmt.Errorf("mysql: list comments: %w", err)
	}

	comments := make([]domain.Comment, 0, len(rows))
	for _, r := range rows {
		c, err := toComment(r)
		if err != nil {
			log.Error("failed to parse comment row", slogx.Err(err))
			return nil, err
		}
		comments = append(comments, c)
	}

	log.Info("comments listed", slog.Int("count", len(comments)))
	return comments, nil
}

func toComment(r commentRow) (domain.Comment, error) {
	taskID, err := bytesToUUID(r.TaskID)
	if err != nil {
		return domain.Comment{}, fmt.Errorf("mysql: parse comment task_id: %w", err)
	}
	userID, err := bytesToUUID(r.UserID)
	if err != nil {
		return domain.Comment{}, fmt.Errorf("mysql: parse comment user_id: %w", err)
	}
	return domain.Comment{
		ID:        r.ID,
		TaskID:    taskID,
		UserID:    userID,
		Body:      r.Body,
		CreatedAt: r.CreatedAt,
	}, nil
}
