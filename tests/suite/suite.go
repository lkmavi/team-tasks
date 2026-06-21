//go:build integration

package suite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/lkmavi/team-tasks/internal/app"
	"github.com/lkmavi/team-tasks/internal/config"
	"github.com/lkmavi/team-tasks/pkg/logger"
)

// Suite holds the shared test infrastructure for the entire integration test run.
type Suite struct {
	App     *app.App
	Client  *http.Client
	DB      *sqlx.DB
	baseURL string
	rdb     *redis.Client
}

// shared is the single Suite instance spun up once by TestMain.
var shared *Suite

// truncateOrder respects FK constraints: children before parents.
var truncateOrder = []string{
	"task_history",
	"task_comments",
	"team_members",
	"tasks",
	"teams",
	"users",
}

// Start spins up MySQL and Redis containers once, runs migrations, and starts
// the application server. Call it from TestMain; the returned func tears everything down.
func Start() func() {
	ctx := context.Background()

	mysqlC, err := tcmysql.Run(ctx, "mysql:8.4",
		tcmysql.WithDatabase("team_tasks_test"),
		tcmysql.WithUsername("root"),
		tcmysql.WithPassword("root"),
	)
	must(err, "start mysql container")

	mysqlDSN, err := mysqlC.ConnectionString(ctx, "parseTime=true&loc=UTC&multiStatements=true")
	must(err, "mysql connection string")

	redisC, err := tcredis.Run(ctx, "redis:7.4-alpine")
	must(err, "start redis container")

	redisHost, err := redisC.Host(ctx)
	must(err, "redis host")
	redisPort, err := redisC.MappedPort(ctx, "6379/tcp")
	must(err, "redis port")
	redisAddr := net.JoinHostPort(redisHost, redisPort.Port())

	db, err := sqlx.Open("mysql", mysqlDSN)
	must(err, "open db")
	must(db.Ping(), "ping db")

	driver, err := migratemysql.WithInstance(db.DB, &migratemysql.Config{})
	must(err, "create migrate driver")

	m, err := migrate.NewWithDatabaseInstance("file://"+resolveMigrationsPath(), "mysql", driver)
	must(err, "create migrate instance")
	must(m.Up(), "run migrations")

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})

	dbConn := parseTCPAddr(mysqlDSN)
	freePort := mustFreePort()

	cfg := &config.Config{
		Server: config.Server{
			Port:            freePort,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    10 * time.Second,
			ShutdownTimeout: 5 * time.Second,
		},
		Database: config.Database{
			Host:            dbConn.host,
			Port:            dbConn.port,
			Name:            "team_tasks_test",
			User:            "root",
			Password:        "root",
			MaxOpenConns:    5,
			MaxIdleConns:    2,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Redis: config.Redis{
			Addr:        redisAddr,
			DB:          0,
			PoolSize:    5,
			TaskListTTL: 5 * time.Minute,
		},
		Auth: config.Auth{
			JWTSecret: "test-integration-secret",
			JWTExpiry: 24 * time.Hour,
		},
		RateLimit: config.RateLimit{
			RequestsPerMinute: 10000,
		},
	}

	a, err := app.New(cfg, logger.Nop())
	must(err, "init app")

	go func() { _ = a.ListenAndServe() }()

	addr := fmt.Sprintf("127.0.0.1:%d", freePort)
	mustWaitForServer(addr)

	shared = &Suite{
		App:     a,
		Client:  &http.Client{Timeout: 10 * time.Second},
		DB:      db,
		rdb:     rdb,
		baseURL: "http://" + addr,
	}

	return func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.Shutdown(shutCtx)
		_ = rdb.Close()
		db.Close()
		_ = mysqlC.Terminate(context.Background())
		_ = redisC.Terminate(context.Background())
	}
}

// New truncates all tables and flushes Redis, then returns the shared Suite.
// Each test gets a clean slate without starting new containers.
// Panics if called before Start (i.e., without TestMain).
func New(t *testing.T) (context.Context, *Suite) {
	t.Helper()
	if shared == nil {
		t.Fatal("suite.New called before suite.Start — add TestMain to your test package")
	}
	shared.reset(t)
	return context.Background(), shared
}

// reset truncates every table and flushes Redis before each test.
// Truncation happens here (not in t.Cleanup) so leftover state from a
// failed test remains inspectable via s.DB after the test exits.
func (s *Suite) reset(t *testing.T) {
	t.Helper()
	_, err := s.DB.Exec("SET FOREIGN_KEY_CHECKS = 0")
	require.NoError(t, err, "disable FK checks")
	for _, tbl := range truncateOrder {
		_, err = s.DB.Exec("TRUNCATE TABLE " + tbl)
		require.NoError(t, err, "truncate %s", tbl)
	}
	_, err = s.DB.Exec("SET FOREIGN_KEY_CHECKS = 1")
	require.NoError(t, err, "re-enable FK checks")
	require.NoError(t, s.rdb.FlushDB(context.Background()).Err(), "flush redis")
}

// URL builds a full URL to the test server.
func (s *Suite) URL(path string) string { return s.baseURL + path }

// POST sends a JSON POST request and returns the response.
func (s *Suite) POST(t *testing.T, path string, body any, token string) *http.Response {
	t.Helper()
	return s.do(t, http.MethodPost, path, body, token)
}

// PUT sends a JSON PUT request.
func (s *Suite) PUT(t *testing.T, path string, body any, token string) *http.Response {
	t.Helper()
	return s.do(t, http.MethodPut, path, body, token)
}

// GET sends a GET request.
func (s *Suite) GET(t *testing.T, path string, token string) *http.Response {
	t.Helper()
	return s.do(t, http.MethodGet, path, nil, token)
}

func (s *Suite) do(t *testing.T, method, path string, body any, token string) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, s.URL(path), bodyReader)
	require.NoError(t, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := s.Client.Do(req)
	require.NoError(t, err)
	return resp
}

// Decode reads and JSON-decodes the response body into dst.
// The caller is responsible for closing resp.Body.
func Decode(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
}

func must(err error, msg string) {
	if err != nil {
		panic(fmt.Sprintf("suite.Start: %s: %v", msg, err))
	}
}

func mustWaitForServer(addr string) {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	panic(fmt.Sprintf("suite.Start: server at %s did not start in time", addr))
}

func mustFreePort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func resolveMigrationsPath() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "../../migrations"
	}
	// suite.go lives at tests/suite/suite.go — two levels up to project root.
	return filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
}

type tcpAddr struct {
	host string
	port int
}

// parseTCPAddr extracts host and port from a DSN like root:root@tcp(HOST:PORT)/db?...
func parseTCPAddr(dsn string) tcpAddr {
	const prefix = "@tcp("
	start := 0
	for i := range dsn {
		if dsn[i:] >= prefix && dsn[i:i+len(prefix)] == prefix {
			start = i + len(prefix)
			break
		}
	}
	end := start
	for i := start; i < len(dsn); i++ {
		if dsn[i] == ')' {
			end = i
			break
		}
	}
	host, portStr, err := net.SplitHostPort(dsn[start:end])
	if err != nil {
		return tcpAddr{"localhost", 3306}
	}
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	return tcpAddr{host, port}
}
