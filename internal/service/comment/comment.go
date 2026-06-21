package comment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/lkmavi/team-tasks/internal/domain"
	"github.com/lkmavi/team-tasks/pkg/slogx"
)

// CommentSaver persists a new comment and returns it with its generated ID.
type CommentSaver interface {
	SaveComment(ctx context.Context, c domain.Comment) (domain.Comment, error)
}

// CommentLister retrieves all comments for a task.
type CommentLister interface {
	ListComments(ctx context.Context, taskID uuid.UUID) ([]domain.Comment, error)
}

// TaskGetter retrieves a task by its primary key.
type TaskGetter interface {
	GetTaskByID(ctx context.Context, id uuid.UUID) (domain.Task, error)
}

// MemberChecker verifies that a user belongs to a team and returns their role.
type MemberChecker interface {
	GetMemberRole(ctx context.Context, teamID, userID uuid.UUID) (domain.Role, error)
}

// Service handles adding and listing comments on tasks.
type Service struct {
	saver   CommentSaver
	lister  CommentLister
	tasks   TaskGetter
	members MemberChecker
}

// New creates a comment Service with the given storage adapters.
func New(saver CommentSaver, lister CommentLister, tasks TaskGetter, members MemberChecker) *Service {
	return &Service{saver: saver, lister: lister, tasks: tasks, members: members}
}

func (s *Service) Add(ctx context.Context, callerID, taskID uuid.UUID, body string) (domain.Comment, error) {
	const op = "comment.Service.Add"
	log := slogx.FromContext(ctx).With(slog.String("op", op), slog.String("task_id", taskID.String()))
	log.Info("adding comment")

	task, err := s.tasks.GetTaskByID(ctx, taskID)
	if err != nil {
		log.Error("failed to get task", slogx.Err(err))
		return domain.Comment{}, fmt.Errorf("%s: get task: %w", op, err)
	}

	_, err = s.members.GetMemberRole(ctx, task.TeamID, callerID)
	if errors.Is(err, domain.ErrNotFound) {
		log.Warn("caller is not a team member")
		return domain.Comment{}, domain.ErrForbidden
	}
	if err != nil {
		log.Error("failed to check membership", slogx.Err(err))
		return domain.Comment{}, fmt.Errorf("%s: get member role: %w", op, err)
	}

	c, err := s.saver.SaveComment(ctx, domain.Comment{TaskID: taskID, UserID: callerID, Body: body})
	if err != nil {
		log.Error("failed to save comment", slogx.Err(err))
		return domain.Comment{}, fmt.Errorf("%s: save comment: %w", op, err)
	}

	log.Info("comment added", slog.Int64("comment_id", c.ID))
	return c, nil
}

func (s *Service) List(ctx context.Context, callerID, taskID uuid.UUID) ([]domain.Comment, error) {
	const op = "comment.Service.List"
	log := slogx.FromContext(ctx).With(slog.String("op", op), slog.String("task_id", taskID.String()))
	log.Info("listing comments")

	task, err := s.tasks.GetTaskByID(ctx, taskID)
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

	comments, err := s.lister.ListComments(ctx, taskID)
	if err != nil {
		log.Error("failed to list comments", slogx.Err(err))
		return nil, fmt.Errorf("%s: list comments: %w", op, err)
	}

	log.Info("comments listed", slog.Int("count", len(comments)))
	return comments, nil
}
