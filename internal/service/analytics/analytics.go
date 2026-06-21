package analytics

import (
	"context"
	"fmt"

	"github.com/lkmavi/team-tasks/internal/domain"
)

// TeamSummaryReader provides aggregated member-count and activity data per team.
type TeamSummaryReader interface {
	TeamSummaries(ctx context.Context) ([]domain.TeamSummary, error)
}

// TopContributorReader provides the top-3 task creators per team via a window function.
type TopContributorReader interface {
	TopContributors(ctx context.Context) ([]domain.TopContributor, error)
}

// OrphanTaskReader finds tasks whose assignee is no longer a team member.
type OrphanTaskReader interface {
	OrphanTasks(ctx context.Context) ([]domain.OrphanTask, error)
}

// Service exposes the three analytical queries from the plan.
type Service struct {
	summaries    TeamSummaryReader
	contributors TopContributorReader
	orphans      OrphanTaskReader
}

// New creates an analytics Service backed by the provided storage readers.
func New(summaries TeamSummaryReader, contributors TopContributorReader, orphans OrphanTaskReader) *Service {
	return &Service{summaries: summaries, contributors: contributors, orphans: orphans}
}

func (s *Service) TeamSummaries(ctx context.Context) ([]domain.TeamSummary, error) {
	result, err := s.summaries.TeamSummaries(ctx)
	if err != nil {
		return nil, fmt.Errorf("analytics: team summaries: %w", err)
	}
	return result, nil
}

func (s *Service) TopContributors(ctx context.Context) ([]domain.TopContributor, error) {
	result, err := s.contributors.TopContributors(ctx)
	if err != nil {
		return nil, fmt.Errorf("analytics: top contributors: %w", err)
	}
	return result, nil
}

func (s *Service) OrphanTasks(ctx context.Context) ([]domain.OrphanTask, error) {
	result, err := s.orphans.OrphanTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("analytics: orphan tasks: %w", err)
	}
	return result, nil
}
