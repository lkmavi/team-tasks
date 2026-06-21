package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/lkmavi/team-tasks/internal/domain"
	"github.com/lkmavi/team-tasks/pkg/slogx"
)

// TeamStorage implements team and membership persistence backed by MySQL.
type TeamStorage struct {
	db *sqlx.DB
}

// NewTeamStorage creates a TeamStorage using the provided database connection.
func NewTeamStorage(db *sqlx.DB) *TeamStorage {
	return &TeamStorage{db: db}
}

type teamRow struct {
	ID        []byte    `db:"id"`
	Name      string    `db:"name"`
	CreatedBy []byte    `db:"created_by"`
	CreatedAt time.Time `db:"created_at"`
}

func (s *TeamStorage) CreateTeamWithOwner(ctx context.Context, team domain.Team, ownerID uuid.UUID) error {
	const op = "mysql.TeamStorage.CreateTeamWithOwner"
	log := slogx.FromContext(ctx).With(slog.String("op", op), slog.String("team_id", team.ID.String()))
	log.Info("creating team with owner")

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx,
		"INSERT INTO teams (id, name, created_by, created_at) VALUES (?, ?, ?, ?)",
		uuidToBytes(team.ID), team.Name, uuidToBytes(team.CreatedBy), team.CreatedAt,
	)
	if err != nil {
		log.Error("failed to insert team", slogx.Err(err))
		return fmt.Errorf("mysql: insert team: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, ?)`,
		uuidToBytes(team.ID), uuidToBytes(ownerID), string(domain.RoleOwner),
	)
	if err != nil {
		log.Error("failed to insert owner member", slogx.Err(err))
		return fmt.Errorf("mysql: insert owner member: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit: %w", err)
	}

	log.Info("team created with owner")
	return nil
}

func (s *TeamStorage) GetTeamsByUser(ctx context.Context, userID uuid.UUID) ([]domain.Team, error) {
	const op = "mysql.TeamStorage.GetTeamsByUser"
	log := slogx.FromContext(ctx).With(slog.String("op", op), slog.String("user_id", userID.String()))
	log.Info("fetching teams for user")

	var rows []teamRow
	err := s.db.SelectContext(ctx, &rows, `
		SELECT t.id, t.name, t.created_by, t.created_at
		FROM teams t
		INNER JOIN team_members tm ON tm.team_id = t.id
		WHERE tm.user_id = ?`,
		uuidToBytes(userID),
	)
	if err != nil {
		log.Error("failed to fetch teams", slogx.Err(err))
		return nil, fmt.Errorf("mysql: get teams by user: %w", err)
	}

	teams := make([]domain.Team, 0, len(rows))
	for _, r := range rows {
		t, err := toTeam(r)
		if err != nil {
			log.Error("failed to parse team row", slogx.Err(err))
			return nil, err
		}
		teams = append(teams, t)
	}

	log.Info("teams fetched", slog.Int("count", len(teams)))
	return teams, nil
}

func (s *TeamStorage) GetTeamByID(ctx context.Context, id uuid.UUID) (domain.Team, error) {
	const op = "mysql.TeamStorage.GetTeamByID"
	log := slogx.FromContext(ctx).With(slog.String("op", op), slog.String("team_id", id.String()))
	log.Info("fetching team by id")

	var row teamRow
	err := s.db.GetContext(ctx, &row,
		"SELECT id, name, created_by, created_at FROM teams WHERE id = ?",
		uuidToBytes(id),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("team not found")
			return domain.Team{}, domain.ErrNotFound
		}
		log.Error("failed to fetch team", slogx.Err(err))
		return domain.Team{}, fmt.Errorf("mysql: get team by id: %w", err)
	}

	t, err := toTeam(row)
	if err != nil {
		log.Error("failed to parse team row", slogx.Err(err))
		return domain.Team{}, err
	}

	log.Info("team fetched")
	return t, nil
}

func (s *TeamStorage) SaveMember(ctx context.Context, teamID, userID uuid.UUID, role domain.Role) error {
	const op = "mysql.TeamStorage.SaveMember"
	log := slogx.FromContext(ctx).With(
		slog.String("op", op),
		slog.String("team_id", teamID.String()),
		slog.String("user_id", userID.String()),
		slog.String("role", string(role)),
	)
	log.Info("saving team member")

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE role = VALUES(role)`,
		uuidToBytes(teamID), uuidToBytes(userID), string(role),
	)
	if err != nil {
		log.Error("failed to save member", slogx.Err(err))
		return fmt.Errorf("mysql: save member: %w", err)
	}

	log.Info("member saved")
	return nil
}

func (s *TeamStorage) GetMemberRole(ctx context.Context, teamID, userID uuid.UUID) (domain.Role, error) {
	const op = "mysql.TeamStorage.GetMemberRole"
	log := slogx.FromContext(ctx).With(
		slog.String("op", op),
		slog.String("team_id", teamID.String()),
		slog.String("user_id", userID.String()),
	)
	log.Info("fetching member role")

	var role string
	err := s.db.GetContext(ctx, &role,
		"SELECT role FROM team_members WHERE team_id = ? AND user_id = ?",
		uuidToBytes(teamID), uuidToBytes(userID),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("member not found")
			return "", domain.ErrNotFound
		}
		log.Error("failed to fetch member role", slogx.Err(err))
		return "", fmt.Errorf("mysql: get member role: %w", err)
	}

	log.Info("member role fetched", slog.String("role", role))
	return domain.Role(role), nil
}

func toTeam(r teamRow) (domain.Team, error) {
	id, err := bytesToUUID(r.ID)
	if err != nil {
		return domain.Team{}, fmt.Errorf("mysql: parse team id: %w", err)
	}
	createdBy, err := bytesToUUID(r.CreatedBy)
	if err != nil {
		return domain.Team{}, fmt.Errorf("mysql: parse team created_by: %w", err)
	}
	return domain.Team{ID: id, Name: r.Name, CreatedBy: createdBy, CreatedAt: r.CreatedAt}, nil
}
