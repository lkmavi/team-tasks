package handler

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=../../api/v1/oapi-codegen.yaml ../../api/v1/openapi.yaml

import (
	"context"

	"github.com/google/uuid"

	"github.com/lkmavi/team-tasks/internal/domain"
)

// AuthService is the interface the handler requires from the auth service.
type AuthService interface {
	Register(ctx context.Context, email, name, password string) error
	Login(ctx context.Context, email, password string) (string, error)
}

// TeamService is the interface the handler requires from the team service.
type TeamService interface {
	Create(ctx context.Context, ownerID uuid.UUID, name string) (domain.Team, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]domain.Team, error)
	Invite(ctx context.Context, callerID, teamID, targetUserID uuid.UUID) error
}

// TaskService is the interface the handler requires from the task service.
type TaskService interface {
	Create(ctx context.Context, creatorID uuid.UUID, input domain.CreateTaskInput) (domain.Task, error)
	List(ctx context.Context, callerID uuid.UUID, filter domain.TaskFilter) ([]domain.Task, int, error)
	Update(ctx context.Context, callerID, taskID uuid.UUID, input domain.UpdateTaskInput) (domain.Task, error)
	History(ctx context.Context, callerID, taskID uuid.UUID) ([]domain.TaskHistory, error)
}

// CommentService is the interface the handler requires from the comment service.
type CommentService interface {
	Add(ctx context.Context, callerID, taskID uuid.UUID, body string) (domain.Comment, error)
	List(ctx context.Context, callerID, taskID uuid.UUID) ([]domain.Comment, error)
}

// AnalyticsService is the interface the handler requires from the analytics service.
type AnalyticsService interface {
	TeamSummaries(ctx context.Context) ([]domain.TeamSummary, error)
	TopContributors(ctx context.Context) ([]domain.TopContributor, error)
	OrphanTasks(ctx context.Context) ([]domain.OrphanTask, error)
}

// Handler implements api.StrictServerInterface.
type Handler struct {
	auth      AuthService
	teams     TeamService
	tasks     TaskService
	comments  CommentService
	analytics AnalyticsService
}

// New creates a Handler wiring together all required service dependencies.
func New(auth AuthService, teams TeamService, tasks TaskService, comments CommentService, analytics AnalyticsService) *Handler {
	return &Handler{auth: auth, teams: teams, tasks: tasks, comments: comments, analytics: analytics}
}
