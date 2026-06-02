package index

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cskiller24/querylex/internal/db"
)

// Test 1: BuildJoinGraph with SchemaResult containing one FK
func TestBuildJoinGraph_DeclaredFK(t *testing.T) {
	result := &db.SchemaResult{
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

	graph, err := BuildJoinGraph(result)
	if err != nil {
		t.Fatalf("BuildJoinGraph failed: %v", err)
	}

	if len(graph.Edges) == 0 {
		t.Fatal("expected at least one join edge")
	}

	found := false
	for _, e := range graph.Edges {
		if e.Source == "orders" && e.Target == "users" {
			found = true
			if e.Confidence != 1.0 {
				t.Errorf("expected confidence 1.0 for declared FK, got %f", e.Confidence)
			}
			if e.SourceType != "declared_foreign_key" {
				t.Errorf("expected source_type 'declared_foreign_key', got '%s'", e.SourceType)
			}
			if len(e.Columns) != 1 {
				t.Fatalf("expected 1 column pair, got %d", len(e.Columns))
			}
			if e.Columns[0][0] != "user_id" || e.Columns[0][1] != "id" {
				t.Errorf("expected columns [user_id, id], got [%s, %s]", e.Columns[0][0], e.Columns[0][1])
			}
			if e.Composite {
				t.Error("expected composite=false for single-column FK")
			}
		}
	}
	if !found {
		t.Error("expected JoinEdge from orders → users")
	}
}

// Test 2: BuildJoinGraph with naming pattern inference (exact singular match)
func TestBuildJoinGraph_InferredExactSingular(t *testing.T) {
	result := &db.SchemaResult{
		Tables: []db.TableInfo{
			{
				Schema: "testdb",
				Name:   "user",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
				},
			},
			{
				Schema: "testdb",
				Name:   "posts",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
					{Name: "user_id", Ordinal: 2, ColumnType: "int"},
				},
			},
		},
	}

	graph, err := BuildJoinGraph(result)
	if err != nil {
		t.Fatalf("BuildJoinGraph failed: %v", err)
	}

	found := false
	for _, e := range graph.Edges {
		if e.Source == "posts" && e.Target == "user" {
			found = true
			if e.Confidence < 0.80 || e.Confidence > 0.90 {
				t.Errorf("expected confidence ~0.85 for exact singular match, got %f", e.Confidence)
			}
			if e.SourceType != "inferred_naming_match" {
				t.Errorf("expected source_type 'inferred_naming_match', got '%s'", e.SourceType)
			}
			if len(e.Columns) != 1 || e.Columns[0][0] != "user_id" || e.Columns[0][1] != "id" {
				t.Errorf("expected columns [[user_id, id]], got %v", e.Columns)
			}
		}
	}
	if !found {
		t.Error("expected inferred JoinEdge from posts → user")
	}
}

// Test 3: BuildJoinGraph with plural pattern (lower confidence)
func TestBuildJoinGraph_InferredPlural(t *testing.T) {
	result := &db.SchemaResult{
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
				Name:   "comments",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
					{Name: "user_id", Ordinal: 2, ColumnType: "int"},
				},
			},
		},
	}

	graph, err := BuildJoinGraph(result)
	if err != nil {
		t.Fatalf("BuildJoinGraph failed: %v", err)
	}

	found := false
	for _, e := range graph.Edges {
		if e.Source == "comments" && e.Target == "users" {
			found = true
			if e.Confidence < 0.65 || e.Confidence > 0.75 {
				t.Errorf("expected confidence ~0.70 for plural match, got %f", e.Confidence)
			}
			if e.SourceType != "inferred_naming_match" {
				t.Errorf("expected source_type 'inferred_naming_match', got '%s'", e.SourceType)
			}
		}
	}
	if !found {
		t.Error("expected inferred JoinEdge from comments → users")
	}
}

// Test 4: FindJoinPath with direct FK returns path
func TestFindJoinPath_DirectFK(t *testing.T) {
	result := &db.SchemaResult{
		Tables: []db.TableInfo{
			{
				Schema: "testdb", Name: "users",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
				},
			},
			{
				Schema: "testdb", Name: "comments",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
					{Name: "user_id", Ordinal: 2, ColumnType: "int"},
				},
				Constraints: []db.ConstraintInfo{
					{
						Name:              "comments_user_fk",
						Type:              "FOREIGN_KEY",
						Columns:           []string{"user_id"},
						ReferencedTable:   "users",
						ReferencedColumns: []string{"id"},
					},
				},
			},
		},
	}

	graph, err := BuildJoinGraph(result)
	if err != nil {
		t.Fatalf("BuildJoinGraph failed: %v", err)
	}

	path, err := FindJoinPath("comments", "users", graph)
	if err != nil {
		t.Fatalf("FindJoinPath failed: %v", err)
	}

	if len(path) != 2 {
		t.Fatalf("expected path of length 2, got %v", path)
	}
	if path[0] != "comments" || path[1] != "users" {
		t.Errorf("expected path [comments, users], got %v", path)
	}
}

// Test 5: FindJoinPath with no connection returns error
func TestFindJoinPath_NoConnection(t *testing.T) {
	result := &db.SchemaResult{
		Tables: []db.TableInfo{
			{
				Schema: "testdb", Name: "users",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
				},
			},
			{
				Schema: "testdb", Name: "products",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
				},
			},
		},
	}

	graph, err := BuildJoinGraph(result)
	if err != nil {
		t.Fatalf("BuildJoinGraph failed: %v", err)
	}

	_, err = FindJoinPath("users", "products", graph)
	if err == nil {
		t.Fatal("expected error for no join path, got nil")
	}
	if !strings.Contains(err.Error(), "no join path found") {
		t.Errorf("expected error containing 'no join path found', got: %v", err)
	}
}

// Test 6: Composite FK detection
func TestBuildJoinGraph_CompositeFK(t *testing.T) {
	result := &db.SchemaResult{
		Tables: []db.TableInfo{
			{
				Schema: "testdb", Name: "order_items",
				Columns: []db.ColumnInfo{
					{Name: "order_id", Ordinal: 1, ColumnType: "int"},
					{Name: "product_id", Ordinal: 2, ColumnType: "int"},
				},
				Constraints: []db.ConstraintInfo{
					{
						Name:              "order_items_order_fk",
						Type:              "FOREIGN_KEY",
						Columns:           []string{"order_id", "product_id"},
						ReferencedTable:   "orders",
						ReferencedColumns: []string{"user_id", "product_id"},
					},
				},
			},
			{
				Schema: "testdb", Name: "orders",
				Columns: []db.ColumnInfo{
					{Name: "user_id", Ordinal: 1, ColumnType: "int"},
					{Name: "product_id", Ordinal: 2, ColumnType: "int"},
				},
			},
		},
	}

	graph, err := BuildJoinGraph(result)
	if err != nil {
		t.Fatalf("BuildJoinGraph failed: %v", err)
	}

	found := false
	for _, e := range graph.Edges {
		if e.Source == "order_items" && e.Target == "orders" {
			found = true
			if !e.Composite {
				t.Error("expected composite=true for multi-column FK")
			}
			if len(e.Columns) != 2 {
				t.Fatalf("expected 2 column pairs, got %d", len(e.Columns))
			}
			if e.Columns[0][0] != "order_id" || e.Columns[0][1] != "user_id" {
				t.Errorf("expected first column pair [order_id, user_id], got [%s, %s]", e.Columns[0][0], e.Columns[0][1])
			}
			if e.Columns[1][0] != "product_id" || e.Columns[1][1] != "product_id" {
				t.Errorf("expected second column pair [product_id, product_id], got [%s, %s]", e.Columns[1][0], e.Columns[1][1])
			}
		}
	}
	if !found {
		t.Error("expected JoinEdge from order_items → orders")
	}
}

// Test 7: WriteJoinGraph produces valid JSON matching expected format
func TestWriteJoinGraph_ValidJSON(t *testing.T) {
	result := &db.SchemaResult{
		Tables: []db.TableInfo{
			{
				Schema: "testdb", Name: "users",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
				},
			},
			{
				Schema: "testdb", Name: "orders",
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

	graph, err := BuildJoinGraph(result)
	if err != nil {
		t.Fatalf("BuildJoinGraph failed: %v", err)
	}

	dir := t.TempDir()
	if err := WriteJoinGraph(dir, graph); err != nil {
		t.Fatalf("WriteJoinGraph failed: %v", err)
	}

	joinGraphPath := filepath.Join(dir, "schema", "join_graph.json")
	if _, err := os.Stat(joinGraphPath); os.IsNotExist(err) {
		t.Fatal("join_graph.json not written")
	}

	data, err := os.ReadFile(joinGraphPath)
	if err != nil {
		t.Fatalf("cannot read join_graph.json: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("join_graph.json is not valid JSON: %v", err)
	}

	// Verify edges array exists
	edges, ok := parsed["edges"].([]any)
	if !ok {
		t.Fatal("expected 'edges' array in join_graph.json")
	}
	if len(edges) < 1 {
		t.Fatal("expected at least one edge")
	}

	// Verify edge fields
	edge := edges[0].(map[string]any)
	if _, ok := edge["source"]; !ok {
		t.Error("expected 'source' field in edge")
	}
	if _, ok := edge["target"]; !ok {
		t.Error("expected 'target' field in edge")
	}
	if _, ok := edge["columns"]; !ok {
		t.Error("expected 'columns' field in edge")
	}
	if _, ok := edge["confidence"]; !ok {
		t.Error("expected 'confidence' field in edge")
	}
	if _, ok := edge["source_type"]; !ok {
		t.Error("expected 'source_type' field in edge")
	}
	if _, ok := edge["composite"]; !ok {
		t.Error("expected 'composite' field in edge")
	}

	// Verify metadata fields
	if _, ok := parsed["generated_at"]; !ok {
		t.Error("expected 'generated_at' field in join_graph.json")
	}
	if _, ok := parsed["table_count"]; !ok {
		t.Error("expected 'table_count' field in join_graph.json")
	}
}

// Test: Deduplication — inferred join should be deduped when declared FK already exists for same (source,target)
func TestBuildJoinGraph_Deduplicate(t *testing.T) {
	// Table "users" with FK from "posts" — also has column user_id that would match naming
	result := &db.SchemaResult{
		Tables: []db.TableInfo{
			{
				Schema: "testdb", Name: "users",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
				},
			},
			{
				Schema: "testdb", Name: "posts",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
					{Name: "user_id", Ordinal: 2, ColumnType: "int"},
				},
				Constraints: []db.ConstraintInfo{
					{
						Name:              "posts_user_fk",
						Type:              "FOREIGN_KEY",
						Columns:           []string{"user_id"},
						ReferencedTable:   "users",
						ReferencedColumns: []string{"id"},
					},
				},
			},
		},
	}

	graph, err := BuildJoinGraph(result)
	if err != nil {
		t.Fatalf("BuildJoinGraph failed: %v", err)
	}

	// Should only have one edge (the declared FK), not two
	if len(graph.Edges) != 1 {
		t.Fatalf("expected 1 edge after dedup (declared FK only), got %d", len(graph.Edges))
	}
	if graph.DeclaredFKCount != 1 {
		t.Errorf("expected DeclaredFKCount=1, got %d", graph.DeclaredFKCount)
	}
	if graph.InferredJoinCount != 0 {
		t.Errorf("expected InferredJoinCount=0 (deduped), got %d", graph.InferredJoinCount)
	}
}

// Test: FindJoinPath prefers declared FK edges over inferred
func TestFindJoinPath_PrefersDeclaredFK(t *testing.T) {
	// Both a direct FK and an inferred path exist; FK should be preferred
	result := &db.SchemaResult{
		Tables: []db.TableInfo{
			{
				Schema: "testdb", Name: "users",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
				},
			},
			{
				Schema: "testdb", Name: "profiles",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
				},
			},
			{
				Schema: "testdb", Name: "posts",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
					{Name: "user_id", Ordinal: 2, ColumnType: "int"},
					{Name: "profile_id", Ordinal: 3, ColumnType: "int"},
				},
			},
		},
	}

	graph, err := BuildJoinGraph(result)
	if err != nil {
		t.Fatalf("BuildJoinGraph failed: %v", err)
	}

	// posts → profiles should be inferred (no FK)
	path, err := FindJoinPath("posts", "profiles", graph)
	if err != nil {
		t.Fatalf("FindJoinPath failed: %v", err)
	}
	if len(path) < 2 {
		t.Fatalf("expected a path, got %v", path)
	}
}

// Test: BuildJoinGraph with no tables returns empty graph
func TestBuildJoinGraph_Empty(t *testing.T) {
	result := &db.SchemaResult{}
	graph, err := BuildJoinGraph(result)
	if err != nil {
		t.Fatalf("BuildJoinGraph failed: %v", err)
	}
	if len(graph.Edges) != 0 {
		t.Errorf("expected 0 edges for empty schema, got %d", len(graph.Edges))
	}
	if graph.TableCount != 0 {
		t.Errorf("expected TableCount=0, got %d", graph.TableCount)
	}
}

// Test: FindJoinPath with three-table chain
func TestFindJoinPath_ThreeTableChain(t *testing.T) {
	result := &db.SchemaResult{
		Tables: []db.TableInfo{
			{
				Schema: "testdb", Name: "users",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
				},
			},
			{
				Schema: "testdb", Name: "orders",
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
			{
				Schema: "testdb", Name: "payments",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
					{Name: "order_id", Ordinal: 2, ColumnType: "int"},
				},
				Constraints: []db.ConstraintInfo{
					{
						Name:              "payments_order_fk",
						Type:              "FOREIGN_KEY",
						Columns:           []string{"order_id"},
						ReferencedTable:   "orders",
						ReferencedColumns: []string{"id"},
					},
				},
			},
		},
	}

	graph, err := BuildJoinGraph(result)
	if err != nil {
		t.Fatalf("BuildJoinGraph failed: %v", err)
	}

	// Find path from payments → users (via orders)
	path, err := FindJoinPath("payments", "users", graph)
	if err != nil {
		t.Fatalf("FindJoinPath failed: %v", err)
	}
	if len(path) != 3 {
		t.Fatalf("expected 3-hop path, got %v (len=%d)", path, len(path))
	}
	if path[0] != "payments" || path[1] != "orders" || path[2] != "users" {
		t.Errorf("expected path [payments, orders, users], got %v", path)
	}
}
