package domain

import (
	"time"

	"github.com/google/uuid"
)

// Comment is a message left by a team member on a task.
type Comment struct {
	ID        int64
	TaskID    uuid.UUID
	UserID    uuid.UUID
	Body      string
	CreatedAt time.Time
}

// CreateCommentInput carries the fields required to persist a new comment.
type CreateCommentInput struct {
	TaskID uuid.UUID
	UserID uuid.UUID
	Body   string
}
