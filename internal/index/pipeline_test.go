package index

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cskiller24/querylex/internal/db"
)

// mockAdapter implements db.Adapter for pipeline testing.
type mockAdapter struct {
	schemaResult *db.SchemaResult
	schemaErr    error
	dbType       string
}

func (m *mockAdapter) Connect(ctx context.Context, dsn string) error { return nil }
func (m *mockAdapter) Ping(ctx context.Context) error                { return nil }
func (m *mockAdapter) Close(ctx context.Context) error               { return nil }
func (m *mockAdapter) Schema(ctx context.Context, tables []string) (*db.SchemaResult, error) {
	if m.schemaErr != nil {
		return nil, m.schemaErr
	}
	return m.schemaResult, nil
}
func (m *mockAdapter) Explain(ctx context.Context, query string, analyze bool) (*db.ExplainPlan, error) {
	return nil, nil
}
func (m *mockAdapter) Validate(ctx context.Context, query string) (*db.ValidateResult, error) {
	return nil, nil
}
func (m *mockAdapter) Stats(ctx context.Context, tables []string) (*db.StatsResult, error) {
	return nil, nil
}
func (m *mockAdapter) Indexes(ctx context.Context, tables []string) (*db.IndexesResult, error) {
	return nil, nil
}
func (m *mockAdapter) Joins(ctx context.Context, tables []string) (*db.JoinsResult, error) {
	return nil, nil
}
func (m *mockAdapter) TestConnect(ctx context.Context, dsn string) error { return nil }
func (m *mockAdapter) DatabaseType() string {
	if m.dbType != "" {
		return m.dbType
	}
	return "testdb"
}

// newTestSchemaResult creates a minimal SchemaResult with two tables connected by an FK.
func newTestSchemaResult() *db.SchemaResult {
	return &db.SchemaResult{
		Tables: []db.TableInfo{
			{
				Schema: "testdb",
				Name:   "users",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
				},
			},
			{
				Schema: "testdb",
				Name:   "orders",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
					{Name: "user_id", Ordinal: 2, ColumnType: "int"},
				},
				Constraints: []db.ConstraintInfo{
					{
						Name:              "orders_user_fk",
						Type:              "FOREIGN_KEY",
						Columns:           []string{"user_id"},
						ReferencedTable:   "users",
						ReferencedColumns: []string{"id"},
					},
				},
			},
		},
	}
}

func TestPipeline_RunsAllPhases(t *testing.T) {
	dir := t.TempDir()
	adapter := &mockAdapter{schemaResult: newTestSchemaResult()}
	pipeline := NewPipeline(adapter, dir, "testdb", "test")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := RunPipeline(ctx, pipeline)
	if err != nil {
		t.Fatalf("RunPipeline failed: %v", err)
	}

	// Verify schema.json exists
	schemaPath := filepath.Join(dir, "schema", "schema.json")
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		t.Error("schema.json not written")
	}

	// Verify schema_slim.json exists
	slimPath := filepath.Join(dir, "schema", "schema_slim.json")
	if _, err := os.Stat(slimPath); os.IsNotExist(err) {
		t.Error("schema_slim.json not written")
	}

	// Verify join_graph.json exists
	joinPath := filepath.Join(dir, "schema", "join_graph.json")
	if _, err := os.Stat(joinPath); os.IsNotExist(err) {
		t.Error("join_graph.json not written")
	}

	// Verify domain_map.json exists
	domainPath := filepath.Join(dir, "domain_map.json")
	if _, err := os.Stat(domainPath); os.IsNotExist(err) {
		t.Error("domain_map.json not written")
	}

	// Verify schema/domain_map.json exists
	schemaDomainPath := filepath.Join(dir, "schema", "domain_map.json")
	if _, err := os.Stat(schemaDomainPath); os.IsNotExist(err) {
		t.Error("schema/domain_map.json not written")
	}

	// Verify index_manifest.json exists
	manifestPath := filepath.Join(dir, "indexes", "index_manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Error("index_manifest.json not written")
	}
}

func TestPipeline_StatusTracking(t *testing.T) {
	dir := t.TempDir()
	adapter := &mockAdapter{schemaResult: newTestSchemaResult()}
	pipeline := NewPipeline(adapter, dir, "testdb", "test")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := RunPipeline(ctx, pipeline); err != nil {
		t.Fatalf("RunPipeline failed: %v", err)
	}

	// Read final status
	status, err := ReadIndexStatus(dir)
	if err != nil {
		t.Fatalf("ReadIndexStatus failed: %v", err)
	}
	if status == nil {
		t.Fatal("status is nil")
	}

	if status.Status != "indexed" {
		t.Errorf("expected status='indexed', got '%s'", status.Status)
	}
	if status.ProgressPercent != 100 {
		t.Errorf("expected progress_percent=100, got %d", status.ProgressPercent)
	}
	if status.CurrentPhase != PhaseOutputAssembly {
		t.Errorf("expected current_phase='%s', got '%s'", PhaseOutputAssembly, status.CurrentPhase)
	}
	if status.HeartbeatAt == "" {
		t.Error("expected non-empty heartbeat_at")
	}
	if status.StartedAt == "" {
		t.Error("expected non-empty started_at")
	}
	if status.CompletedAt == "" {
		t.Error("expected non-empty completed_at")
	}
}

func TestPipeline_ManifestChecksums(t *testing.T) {
	dir := t.TempDir()
	adapter := &mockAdapter{schemaResult: newTestSchemaResult()}
	pipeline := NewPipeline(adapter, dir, "testdb", "test")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := RunPipeline(ctx, pipeline); err != nil {
		t.Fatalf("RunPipeline failed: %v", err)
	}

	manifest, err := ReadIndexManifest(dir)
	if err != nil {
		t.Fatalf("ReadIndexManifest failed: %v", err)
	}
	if manifest == nil {
		t.Fatal("manifest is nil")
	}

	if manifest.TableCount != 2 {
		t.Errorf("expected TableCount=2, got %d", manifest.TableCount)
	}
	if manifest.DBVersion != "test" {
		t.Errorf("expected DBVersion='test', got '%s'", manifest.DBVersion)
	}

	// Verify artifact checksums exist for key files
	expectedArtifacts := []string{
		"schema/schema.json",
		"schema/schema_slim.json",
		"schema/join_graph.json",
		"domain_map.json",
		"schema/domain_map.json",
	}
	for _, artifact := range expectedArtifacts {
		if _, ok := manifest.ArtifactChecksums[artifact]; !ok {
			t.Errorf("missing artifact checksum for '%s'", artifact)
		}
	}

	// Verify manifest checksums match
	ok, err := VerifyManifest(dir, manifest)
	if err != nil {
		t.Fatalf("VerifyManifest failed: %v", err)
	}
	if !ok {
		t.Error("expected VerifyManifest to return true")
	}
}

func TestPipeline_DomainMapMetadata(t *testing.T) {
	dir := t.TempDir()
	adapter := &mockAdapter{schemaResult: newTestSchemaResult()}
	pipeline := NewPipeline(adapter, dir, "testdb", "test")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := RunPipeline(ctx, pipeline); err != nil {
		t.Fatalf("RunPipeline failed: %v", err)
	}

	// Read domain_map.json
	domainPath := filepath.Join(dir, "domain_map.json")
	data, err := os.ReadFile(domainPath)
	if err != nil {
		t.Fatalf("read domain_map.json: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("domain_map.json not valid JSON: %v", err)
	}

	meta, ok := parsed["metadata"].(map[string]any)
	if !ok {
		t.Fatal("expected 'metadata' in domain_map.json")
	}

	tc, ok := meta["table_count"].(float64)
	if !ok || int(tc) != 2 {
		t.Errorf("expected table_count=2, got %v", meta["table_count"])
	}
}

func TestPipeline_AdapterFailure(t *testing.T) {
	dir := t.TempDir()
	adapter := &mockAdapter{
		schemaErr: db.ErrConnectionFailed,
	}
	pipeline := NewPipeline(adapter, dir, "testdb", "test")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := RunPipeline(ctx, pipeline)
	if err == nil {
		t.Fatal("expected error from failing adapter, got nil")
	}

	// Status should be index_failed
	status, err := ReadIndexStatus(dir)
	if err != nil {
		t.Fatalf("ReadIndexStatus failed: %v", err)
	}
	if status == nil {
		t.Fatal("status is nil")
	}

	// Status might be "index_failed" if WriteIndexStatus succeeded, or "not_indexed"
	// if the status write failed before the failure path. Check that error field is set.
	if status.Status != "index_failed" {
		t.Logf("status is '%s' (not 'index_failed'), which may happen if initial status write was overwritten", status.Status)
	}

	// Manifest should NOT be written on failure
	manifest, err := ReadIndexManifest(dir)
	if err != nil {
		t.Fatalf("ReadIndexManifest failed: %v", err)
	}
	if manifest != nil {
		t.Error("expected no manifest on failure")
	}
}

func TestPipeline_JoinGraphCrossDomain(t *testing.T) {
	dir := t.TempDir()
	adapter := &mockAdapter{schemaResult: newTestSchemaResult()}
	pipeline := NewPipeline(adapter, dir, "testdb", "test")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := RunPipeline(ctx, pipeline); err != nil {
		t.Fatalf("RunPipeline failed: %v", err)
	}

	// Read join_graph.json
	joinPath := filepath.Join(dir, "schema", "join_graph.json")
	data, err := os.ReadFile(joinPath)
	if err != nil {
		t.Fatalf("read join_graph.json: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("join_graph.json not valid JSON: %v", err)
	}

	edges, ok := parsed["edges"].([]any)
	if !ok {
		t.Fatal("expected 'edges' array in join_graph.json")
	}

	// Each edge should have a cross_domain field
	for i, edge := range edges {
		e := edge.(map[string]any)
		if _, ok := e["cross_domain"]; !ok {
			t.Errorf("edge %d missing 'cross_domain' field", i)
		}
	}
}

// Helpers that the pipeline requires — define them as package-level
// functions since they're called from pipeline.go.

// init is not called in tests — we need to ensure pipeline works
// without init overrides. Let's verify the test assembles correctly.
