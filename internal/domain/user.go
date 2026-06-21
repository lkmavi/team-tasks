package domain

import (
	"time"

	"github.com/google/uuid"
)

// User is a registered account in the system.
type User struct {
	ID        uuid.UUID
	Email     string
	Name      string
	Password  string // bcrypt hash
	CreatedAt time.Time
}
