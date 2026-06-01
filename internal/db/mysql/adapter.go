package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"time"

	"github.com/querylex/querylex/internal/db"
	_ "github.com/go-sql-driver/mysql"
)

func init() {
	db.Register("mysql", func(dsn string) (db.Adapter, error) {
		return &MySQLAdapter{dsn: dsn}, nil
	})
}

type MySQLAdapter struct {
	dsn  string
	conn *sql.DB
}

func (a *MySQLAdapter) Connect(ctx context.Context, dsn string) error {
	if a.conn != nil {
		if err := a.conn.PingContext(ctx); err == nil {
			return nil
		}
		a.conn.Close()
	}

	if dsn != "" {
		a.dsn = dsn
	}

	conn, err := sql.Open("mysql", a.dsn)
	if err != nil {
		return fmt.Errorf("mysql connect: %w", err)
	}

	conn.SetMaxOpenConns(1)
	conn.SetConnMaxLifetime(5 * time.Minute)
	conn.SetConnMaxIdleTime(1 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := conn.PingContext(pingCtx); err != nil {
		conn.Close()
		return fmt.Errorf("%w: mysql ping: %w", db.ErrConnectionFailed, err)
	}

	a.conn = conn
	return nil
}

func (a *MySQLAdapter) Ping(ctx context.Context) error {
	if a.conn == nil {
		return fmt.Errorf("%w: not connected", db.ErrConnectionFailed)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := a.conn.PingContext(pingCtx); err != nil {
		return fmt.Errorf("%w: mysql ping: %w", db.ErrConnectionFailed, err)
	}
	return nil
}

func (a *MySQLAdapter) Close(ctx context.Context) error {
	if a.conn != nil {
		return a.conn.Close()
	}
	return nil
}

func (a *MySQLAdapter) Schema(ctx context.Context, tables []string) (any, error) {
	return nil, db.ErrNotImplemented
}

func (a *MySQLAdapter) Explain(ctx context.Context, query string, analyze bool) (any, error) {
	return nil, db.ErrNotImplemented
}

func (a *MySQLAdapter) Validate(ctx context.Context, query string) (any, error) {
	return nil, db.ErrNotImplemented
}

func (a *MySQLAdapter) Stats(ctx context.Context, tables []string) (any, error) {
	return nil, db.ErrNotImplemented
}

func (a *MySQLAdapter) Indexes(ctx context.Context, tables []string) (any, error) {
	return nil, db.ErrNotImplemented
}

func (a *MySQLAdapter) Joins(ctx context.Context, tables []string) (any, error) {
	return nil, db.ErrNotImplemented
}

func (a *MySQLAdapter) DatabaseType() string {
	return "mysql"
}

func BuildDSN(host string, port int, database, username, password string, sslMode string) string {
	params := url.Values{}
	if sslMode != "" && sslMode != "disable" {
		params.Set("tls", sslMode)
	}
	if sslMode == "disable" {
		params.Set("tls", "false")
	}
	paramStr := params.Encode()
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", username, password, host, port, database)
	if paramStr != "" {
		dsn += "?" + paramStr
	}
	return dsn
}
