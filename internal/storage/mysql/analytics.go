package mysql

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"

	"github.com/lkmavi/team-tasks/internal/domain"
	"github.com/lkmavi/team-tasks/pkg/slogx"
)

// AnalyticsStorage implements the three analytical queries backed by MySQL.
type AnalyticsStorage struct {
	db *sqlx.DB
}

// NewAnalyticsStorage creates an AnalyticsStorage using the provided database connection.
func NewAnalyticsStorage(db *sqlx.DB) *AnalyticsStorage {
	return &AnalyticsStorage{db: db}
}

type teamSummaryRow struct {
	ID            []byte `db:"id"`
	Name          string `db:"name"`
	MemberCount   int    `db:"member_count"`
	DoneLast7Days int    `db:"done_last_7_days"`
}

type topContributorRow struct {
	TeamID     []byte `db:"team_id"`
	UserID     []byte `db:"user_id"`
	TaskCount  int    `db:"task_count"`
	RankInTeam int    `db:"rank_in_team"`
}

type orphanTaskRow struct {
	ID         []byte `db:"id"`
	Title      string `db:"title"`
	TeamID     []byte `db:"team_id"`
	AssigneeID []byte `db:"assignee_id"`
}

// TeamSummaries returns member count and completed tasks (last 7 days) for every team.
func (s *AnalyticsStorage) TeamSummaries(ctx context.Context) ([]domain.TeamSummary, error) {
	const op = "mysql.AnalyticsStorage.TeamSummaries"
	log := slogx.FromContext(ctx).With(slog.String("op", op))
	log.Info("querying team summaries")

	var rows []teamSummaryRow
	err := s.db.SelectContext(ctx, &rows, `
		SELECT
			t.id,
			t.name,
			COUNT(DISTINCT tm.user_id)  AS member_count,
			COUNT(DISTINCT CASE
				WHEN tk.status = 'done' AND tk.updated_at >= NOW() - INTERVAL 7 DAY
				THEN tk.id END)         AS done_last_7_days
		FROM teams t
		LEFT JOIN team_members tm ON tm.team_id = t.id
		LEFT JOIN tasks tk        ON tk.team_id = t.id
		GROUP BY t.id, t.name`,
	)
	if err != nil {
		log.Error("failed to query team summaries", slogx.Err(err))
		return nil, fmt.Errorf("mysql: team summaries: %w", err)
	}

	result := make([]domain.TeamSummary, 0, len(rows))
	for _, r := range rows {
		id, err := bytesToUUID(r.ID)
		if err != nil {
			return nil, fmt.Errorf("mysql: parse summary team id: %w", err)
		}
		result = append(result, domain.TeamSummary{
			TeamID:        id,
			Name:          r.Name,
			MemberCount:   r.MemberCount,
			DoneLast7Days: r.DoneLast7Days,
		})
	}

	log.Info("team summaries fetched", slog.Int("count", len(result)))
	return result, nil
}

// TopContributors returns the top-3 task creators per team over the last 30 days.
func (s *AnalyticsStorage) TopContributors(ctx context.Context) ([]domain.TopContributor, error) {
	const op = "mysql.AnalyticsStorage.TopContributors"
	log := slogx.FromContext(ctx).With(slog.String("op", op))
	log.Info("querying top contributors")

	var rows []topContributorRow
	err := s.db.SelectContext(ctx, &rows, `
		SELECT team_id, user_id, task_count, rank_in_team
		FROM (
			SELECT
				tk.team_id,
				tk.created_by                AS user_id,
				COUNT(*)                     AS task_count,
				RANK() OVER (
					PARTITION BY tk.team_id
					ORDER BY COUNT(*) DESC
				)                            AS rank_in_team
			FROM tasks tk
			WHERE tk.created_at >= NOW() - INTERVAL 30 DAY
			GROUP BY tk.team_id, tk.created_by
		) ranked
		WHERE rank_in_team <= 3`,
	)
	if err != nil {
		log.Error("failed to query top contributors", slogx.Err(err))
		return nil, fmt.Errorf("mysql: top contributors: %w", err)
	}

	result := make([]domain.TopContributor, 0, len(rows))
	for _, r := range rows {
		teamID, err := bytesToUUID(r.TeamID)
		if err != nil {
			return nil, fmt.Errorf("mysql: parse contributor team_id: %w", err)
		}
		userID, err := bytesToUUID(r.UserID)
		if err != nil {
			return nil, fmt.Errorf("mysql: parse contributor user_id: %w", err)
		}
		result = append(result, domain.TopContributor{
			TeamID:     teamID,
			UserID:     userID,
			TaskCount:  r.TaskCount,
			RankInTeam: r.RankInTeam,
		})
	}

	log.Info("top contributors fetched", slog.Int("count", len(result)))
	return result, nil
}

// OrphanTasks returns tasks whose assignee is no longer a member of the task's team.
func (s *AnalyticsStorage) OrphanTasks(ctx context.Context) ([]domain.OrphanTask, error) {
	const op = "mysql.AnalyticsStorage.OrphanTasks"
	log := slogx.FromContext(ctx).With(slog.String("op", op))
	log.Info("querying orphan tasks")

	var rows []orphanTaskRow
	err := s.db.SelectContext(ctx, &rows, `
		SELECT tk.id, tk.title, tk.team_id, tk.assignee_id
		FROM tasks tk
		WHERE tk.assignee_id IS NOT NULL
		  AND NOT EXISTS (
			  SELECT 1 FROM team_members tm
			  WHERE tm.team_id = tk.team_id
				AND tm.user_id = tk.assignee_id
		  )`,
	)
	if err != nil {
		log.Error("failed to query orphan tasks", slogx.Err(err))
		return nil, fmt.Errorf("mysql: orphan tasks: %w", err)
	}

	result := make([]domain.OrphanTask, 0, len(rows))
	for _, r := range rows {
		id, err := bytesToUUID(r.ID)
		if err != nil {
			return nil, fmt.Errorf("mysql: parse orphan task id: %w", err)
		}
		teamID, err := bytesToUUID(r.TeamID)
		if err != nil {
			return nil, fmt.Errorf("mysql: parse orphan team_id: %w", err)
		}
		assigneeID, err := bytesToUUID(r.AssigneeID)
		if err != nil {
			return nil, fmt.Errorf("mysql: parse orphan assignee_id: %w", err)
		}
		result = append(result, domain.OrphanTask{
			ID:         id,
			Title:      r.Title,
			TeamID:     teamID,
			AssigneeID: assigneeID,
		})
	}

	log.Info("orphan tasks fetched", slog.Int("count", len(result)))
	return result, nil
}
