package analytics_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/lkmavi/team-tasks/internal/domain"
	. "github.com/lkmavi/team-tasks/internal/service/analytics"
)

var ctx = context.Background()

type mockSummaries struct{ err error }

func (m *mockSummaries) TeamSummaries(_ context.Context) ([]domain.TeamSummary, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []domain.TeamSummary{{TeamID: uuid.New(), Name: "alpha"}}, nil
}

type mockContributors struct{ err error }

func (m *mockContributors) TopContributors(_ context.Context) ([]domain.TopContributor, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []domain.TopContributor{{TeamID: uuid.New(), TaskCount: 5}}, nil
}

type mockOrphans struct{ err error }

func (m *mockOrphans) OrphanTasks(_ context.Context) ([]domain.OrphanTask, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []domain.OrphanTask{{ID: uuid.New(), Title: "orphan"}}, nil
}

func TestTeamSummaries_OK(t *testing.T) {
	svc := New(&mockSummaries{}, &mockContributors{}, &mockOrphans{})
	result, err := svc.TeamSummaries(ctx)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestTeamSummaries_Error(t *testing.T) {
	svc := New(&mockSummaries{err: errors.New("db")}, &mockContributors{}, &mockOrphans{})
	_, err := svc.TeamSummaries(ctx)
	assert.Error(t, err)
}

func TestTopContributors_OK(t *testing.T) {
	svc := New(&mockSummaries{}, &mockContributors{}, &mockOrphans{})
	result, err := svc.TopContributors(ctx)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestTopContributors_Error(t *testing.T) {
	svc := New(&mockSummaries{}, &mockContributors{err: errors.New("db")}, &mockOrphans{})
	_, err := svc.TopContributors(ctx)
	assert.Error(t, err)
}

func TestOrphanTasks_OK(t *testing.T) {
	svc := New(&mockSummaries{}, &mockContributors{}, &mockOrphans{})
	result, err := svc.OrphanTasks(ctx)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestOrphanTasks_Error(t *testing.T) {
	svc := New(&mockSummaries{}, &mockContributors{}, &mockOrphans{err: errors.New("db")})
	_, err := svc.OrphanTasks(ctx)
	assert.Error(t, err)
}
