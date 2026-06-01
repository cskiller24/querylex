package postgresql

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"time"

	"github.com/querylex/querylex/internal/db"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func init() {
	db.Register("postgres", func(dsn string) (db.Adapter, error) {
		return &PostgreSQLAdapter{dsn: dsn}, nil
	})
	db.Register("postgresql", func(dsn string) (db.Adapter, error) {
		return &PostgreSQLAdapter{dsn: dsn}, nil
	})
}

type PostgreSQLAdapter struct {
	dsn  string
	conn *sql.DB
}

func (a *PostgreSQLAdapter) Connect(ctx context.Context, dsn string) error {
	if a.conn != nil {
		if err := a.conn.PingContext(ctx); err == nil {
			return nil
		}
		a.conn.Close()
	}

	if dsn != "" {
		a.dsn = dsn
	}

	conn, err := sql.Open("pgx", a.dsn)
	if err != nil {
		return fmt.Errorf("postgresql connect: %w", err)
	}

	conn.SetMaxOpenConns(1)
	conn.SetConnMaxLifetime(5 * time.Minute)
	conn.SetConnMaxIdleTime(1 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := conn.PingContext(pingCtx); err != nil {
		conn.Close()
		return fmt.Errorf("%w: postgresql ping: %w", db.ErrConnectionFailed, err)
	}

	a.conn = conn
	return nil
}

func (a *PostgreSQLAdapter) Ping(ctx context.Context) error {
	if a.conn == nil {
		return fmt.Errorf("%w: not connected", db.ErrConnectionFailed)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := a.conn.PingContext(pingCtx); err != nil {
		return fmt.Errorf("%w: postgresql ping: %w", db.ErrConnectionFailed, err)
	}
	return nil
}

func (a *PostgreSQLAdapter) Close(ctx context.Context) error {
	if a.conn != nil {
		return a.conn.Close()
	}
	return nil
}

func (a *PostgreSQLAdapter) Schema(ctx context.Context, tables []string) (any, error) {
	return nil, db.ErrNotImplemented
}

func (a *PostgreSQLAdapter) Explain(ctx context.Context, query string, analyze bool) (any, error) {
	return nil, db.ErrNotImplemented
}

func (a *PostgreSQLAdapter) Validate(ctx context.Context, query string) (any, error) {
	return nil, db.ErrNotImplemented
}

func (a *PostgreSQLAdapter) Stats(ctx context.Context, tables []string) (any, error) {
	return nil, db.ErrNotImplemented
}

func (a *PostgreSQLAdapter) Indexes(ctx context.Context, tables []string) (any, error) {
	return nil, db.ErrNotImplemented
}

func (a *PostgreSQLAdapter) Joins(ctx context.Context, tables []string) (any, error) {
	return nil, db.ErrNotImplemented
}

func (a *PostgreSQLAdapter) DatabaseType() string {
	return "postgresql"
}

func BuildDSN(host string, port int, database, username, password string, sslMode string) string {
	u := &url.URL{
		Scheme: "postgres",
		Host:   fmt.Sprintf("%s:%d", host, port),
		Path:   database,
	}
	if username != "" {
		if password != "" {
			u.User = url.UserPassword(username, password)
		} else {
			u.User = url.User(username)
		}
	}
	q := u.Query()
	if sslMode != "" {
		q.Set("sslmode", sslMode)
	}
	u.RawQuery = q.Encode()
	return u.String()
}
