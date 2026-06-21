package domain

import (
	"time"

	"github.com/google/uuid"
)

// Role is the access level a user holds within a team.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// Team represents a group of users that share a task board.
type Team struct {
	ID        uuid.UUID
	Name      string
	CreatedBy uuid.UUID
	CreatedAt time.Time
}
