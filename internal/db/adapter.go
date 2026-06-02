package db

import (
	"context"
	"errors"
)

var ErrNotImplemented = errors.New("not implemented in this adapter version")
var ErrConnectionFailed = errors.New("database connection failed")
var ErrExplainFailed = errors.New("explain plan extraction failed")
var ErrSchemaParseError = errors.New("schema parsing failed")
var ErrUnsafeSQL = errors.New("unsafe SQL rejected")
var ErrInvalidSQL = errors.New("invalid SQL")

type Adapter interface {
	Connect(ctx context.Context, dsn string) error
	Ping(ctx context.Context) error
	Close(ctx context.Context) error

	Schema(ctx context.Context, tables []string) (*SchemaResult, error)
	Explain(ctx context.Context, query string, analyze bool) (*ExplainPlan, error)
	Validate(ctx context.Context, query string) (*ValidateResult, error)
	Stats(ctx context.Context, tables []string) (*StatsResult, error)
	Indexes(ctx context.Context, tables []string) (*IndexesResult, error)
	Joins(ctx context.Context, tables []string) (*JoinsResult, error)

	DatabaseType() string
}
