package app

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/jmoiron/sqlx"

	"github.com/lkmavi/team-tasks/internal/circuitbreaker"
	"github.com/lkmavi/team-tasks/internal/config"
	"github.com/lkmavi/team-tasks/internal/handler"
	apiv1 "github.com/lkmavi/team-tasks/internal/handler/api/v1"
	"github.com/lkmavi/team-tasks/internal/middleware"
	analyticsvc "github.com/lkmavi/team-tasks/internal/service/analytics"
	authsvc "github.com/lkmavi/team-tasks/internal/service/auth"
	commentsvc "github.com/lkmavi/team-tasks/internal/service/comment"
	tasksvc "github.com/lkmavi/team-tasks/internal/service/task"
	teamsvc "github.com/lkmavi/team-tasks/internal/service/team"
	mysqlstore "github.com/lkmavi/team-tasks/internal/storage/mysql"
	redisstore "github.com/lkmavi/team-tasks/internal/storage/redis"
	"github.com/lkmavi/team-tasks/pkg/slogx"
)

// App owns the HTTP server and all long-lived infrastructure connections.
type App struct {
	srv    *http.Server
	db     *sqlx.DB
	rdb    *redis.Client
	cancel context.CancelFunc
}

// ListenAndServe starts the HTTP server. Blocks until the server stops.
func (a *App) ListenAndServe() error { return a.srv.ListenAndServe() }

// Shutdown gracefully stops the server and closes infrastructure connections.
func (a *App) Shutdown(ctx context.Context) error {
	a.cancel()
	err := a.srv.Shutdown(ctx)
	_ = a.db.Close()
	_ = a.rdb.Close()
	return err
}

// Addr returns the listen address (e.g. ":8080").
func (a *App) Addr() string { return a.srv.Addr }

// New wires the full dependency graph and returns a ready-to-start App.
func New(cfg *config.Config, log *slogx.Logger) (*App, error) {
	appCtx, cancel := context.WithCancel(context.Background())

	db, err := mysqlstore.New(&cfg.Database)
	if err != nil {
		cancel()
		return nil, err
	}

	rdb, err := redisstore.New(&cfg.Redis)
	if err != nil {
		cancel()
		_ = db.Close()
		return nil, err
	}

	userStorage := mysqlstore.NewUserStorage(db)
	teamStorage := mysqlstore.NewTeamStorage(db)
	taskStorage := mysqlstore.NewTaskStorage(db)
	commentStorage := mysqlstore.NewCommentStorage(db)
	analyticsStorage := mysqlstore.NewAnalyticsStorage(db)
	taskCache := redisstore.NewTaskCache(rdb)

	emailClient := circuitbreaker.NewEmailClient(cfg.EmailService, cfg.CircuitBreaker)

	authSvc := authsvc.New(userStorage, userStorage, cfg.Auth.JWTSecret, cfg.Auth.JWTExpiry)
	teamSvc := teamsvc.New(teamStorage, teamStorage, userStorage, emailClient)
	taskSvc := tasksvc.New(
		taskStorage, taskStorage, taskStorage, taskStorage,
		teamStorage,
		taskCache,
		cfg.Redis.TaskListTTL,
	)
	commentSvc := commentsvc.New(commentStorage, commentStorage, taskStorage, teamStorage)
	analyticsSvc := analyticsvc.New(analyticsStorage, analyticsStorage, analyticsStorage)

	h := handler.New(authSvc, teamSvc, taskSvc, commentSvc, analyticsSvc)

	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	// RequestID first: all subsequent middleware and handlers get request_id in their logger.
	r.Use(middleware.RequestID(log))
	r.Use(middleware.Metrics)
	// JWT is soft: no token → pass through; invalid token → 401.
	// Must run before RateLimit so the limiter can key by userID.
	r.Use(middleware.JWT(cfg.Auth.JWTSecret))
	r.Use(middleware.RateLimit(appCtx, cfg.RateLimit.RequestsPerMinute))

	r.Handle("/metrics", promhttp.Handler())
	mountV1(r, h)

	return &App{
		srv: &http.Server{
			Addr:         cfg.Server.Addr(),
			Handler:      r,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
		},
		db:     db,
		rdb:    rdb,
		cancel: cancel,
	}, nil
}

// mountV1 registers all /api/v1 routes on r.
// To add a v2 API, implement apiv2.StrictServerInterface and add a mountV2 alongside this function.
func mountV1(r chi.Router, h *handler.Handler) {
	r.Route("/api/v1", func(r chi.Router) {
		apiv1.HandlerFromMux(apiv1.NewStrictHandler(h, nil), r)
	})
}
