package domain

import "github.com/google/uuid"

// TeamSummary aggregates membership and recent activity for a single team.
type TeamSummary struct {
	TeamID        uuid.UUID
	Name          string
	MemberCount   int
	DoneLast7Days int
}

// TopContributor holds the rank and task count for one user within a team.
type TopContributor struct {
	TeamID     uuid.UUID
	UserID     uuid.UUID
	TaskCount  int
	RankInTeam int
}

// OrphanTask is a task whose assignee is no longer a member of the team.
type OrphanTask struct {
	ID         uuid.UUID
	Title      string
	TeamID     uuid.UUID
	AssigneeID uuid.UUID
}
