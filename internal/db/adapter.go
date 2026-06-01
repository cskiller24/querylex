package db

import (
	"context"
	"errors"
)

var ErrNotImplemented = errors.New("not implemented in this adapter version")
var ErrConnectionFailed = errors.New("database connection failed")

type Adapter interface {
	Connect(ctx context.Context, dsn string) error
	Ping(ctx context.Context) error
	Close(ctx context.Context) error

	Schema(ctx context.Context, tables []string) (any, error)
	Explain(ctx context.Context, query string, analyze bool) (any, error)
	Validate(ctx context.Context, query string) (any, error)
	Stats(ctx context.Context, tables []string) (any, error)
	Indexes(ctx context.Context, tables []string) (any, error)
	Joins(ctx context.Context, tables []string) (any, error)

	DatabaseType() string
}
