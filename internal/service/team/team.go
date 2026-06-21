package team

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/lkmavi/team-tasks/internal/domain"
	"github.com/lkmavi/team-tasks/pkg/slogx"
)

// UserGetter retrieves a user by primary key.
type UserGetter interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
}

// TeamSaver persists teams and membership records.
type TeamSaver interface {
	CreateTeamWithOwner(ctx context.Context, team domain.Team, ownerID uuid.UUID) error
	SaveMember(ctx context.Context, teamID, userID uuid.UUID, role domain.Role) error
}

// TeamGetter reads teams and membership data.
type TeamGetter interface {
	GetTeamsByUser(ctx context.Context, userID uuid.UUID) ([]domain.Team, error)
	GetTeamByID(ctx context.Context, id uuid.UUID) (domain.Team, error)
	GetMemberRole(ctx context.Context, teamID, userID uuid.UUID) (domain.Role, error)
}

// Notifier sends out-of-band team invitation messages.
type Notifier interface {
	SendInvite(ctx context.Context, to, teamName string) error
}

// Service handles team management and invitations.
type Service struct {
	teamSaver  TeamSaver
	teamGetter TeamGetter
	users      UserGetter
	notifier   Notifier
}

// New creates a team Service with the given storage adapters and notifier.
func New(
	teamSaver TeamSaver,
	teamGetter TeamGetter,
	users UserGetter,
	notifier Notifier,
) *Service {
	return &Service{
		teamSaver:  teamSaver,
		teamGetter: teamGetter,
		users:      users,
		notifier:   notifier,
	}
}

func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, name string) (domain.Team, error) {
	const op = "team.Service.Create"

	log := slogx.FromContext(ctx).With(
		slog.String("op", op),
		slog.String("owner_id", ownerID.String()),
		slog.String("name", name),
	)

	log.Info("creating team")

	id, err := uuid.NewV7()
	if err != nil {
		return domain.Team{}, fmt.Errorf("%s: generate team id: %w", op, err)
	}

	team := domain.Team{
		ID:        id,
		Name:      name,
		CreatedBy: ownerID,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.teamSaver.CreateTeamWithOwner(ctx, team, ownerID); err != nil {
		log.Error("failed to create team with owner", slogx.Err(err))
		return domain.Team{}, fmt.Errorf("%s: create team with owner: %w", op, err)
	}

	log.Info("team created successfully", slog.String("team_id", team.ID.String()))
	return team, nil
}

func (s *Service) ListForUser(ctx context.Context, userID uuid.UUID) ([]domain.Team, error) {
	const op = "team.Service.ListForUser"

	log := slogx.FromContext(ctx).With(
		slog.String("op", op),
		slog.String("user_id", userID.String()),
	)

	log.Info("listing teams for user")

	teams, err := s.teamGetter.GetTeamsByUser(ctx, userID)
	if err != nil {
		log.Error("failed to get teams", slogx.Err(err))
		return nil, fmt.Errorf("%s: get teams by user: %w", op, err)
	}

	log.Info("teams listed", slog.Int("count", len(teams)))
	return teams, nil
}

func (s *Service) Invite(ctx context.Context, callerID, teamID, targetUserID uuid.UUID) error {
	const op = "team.Service.Invite"

	log := slogx.FromContext(ctx).With(
		slog.String("op", op),
		slog.String("team_id", teamID.String()),
		slog.String("caller_id", callerID.String()),
		slog.String("target_user_id", targetUserID.String()),
	)

	log.Info("inviting user to team")

	team, err := s.teamGetter.GetTeamByID(ctx, teamID)
	if err != nil {
		log.Error("failed to get team", slogx.Err(err))
		return fmt.Errorf("%s: get team: %w", op, err)
	}

	role, err := s.teamGetter.GetMemberRole(ctx, teamID, callerID)
	if errors.Is(err, domain.ErrNotFound) {
		log.Warn("caller is not a team member")
		return domain.ErrForbidden
	}
	if err != nil {
		log.Error("failed to get caller role", slogx.Err(err))
		return fmt.Errorf("%s: get member role: %w", op, err)
	}
	if role == domain.RoleMember {
		log.Warn("caller lacks invite permission", slog.String("role", string(role)))
		return domain.ErrForbidden
	}

	target, err := s.users.GetByID(ctx, targetUserID)
	if err != nil {
		log.Error("failed to get target user", slogx.Err(err))
		return fmt.Errorf("%s: get target user: %w", op, err)
	}

	if err = s.teamSaver.SaveMember(ctx, teamID, targetUserID, domain.RoleMember); err != nil {
		log.Error("failed to save member", slogx.Err(err))
		return fmt.Errorf("%s: save member: %w", op, err)
	}

	// Circuit-breaker failure must not block the invitation.
	if err = s.notifier.SendInvite(ctx, target.Email, team.Name); err != nil {
		log.Warn("invite notification failed (non-fatal)", slogx.Err(err))
	}

	log.Info("user invited successfully")
	return nil
}
