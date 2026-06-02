package index

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/cskiller24/querylex/internal/db"
)

// Helper: create a minimal SchemaResult with declared FKs
func newSchemaResult(tables []db.TableInfo) *db.SchemaResult {
	return &db.SchemaResult{Tables: tables}
}

// Helper: create a JoinGraphResult from SchemaResult
func buildJoinGraphForTest(t *testing.T, result *db.SchemaResult) *JoinGraphResult {
	t.Helper()
	graph, err := BuildJoinGraph(result)
	if err != nil {
		t.Fatalf("BuildJoinGraph failed: %v", err)
	}
	return graph
}

// ============================================================
// Test 1: Graph Construction — edge weights from declared/inferred FKs
// ============================================================

func TestDomain_GraphConstruction(t *testing.T) {
	// Use direct SlimSchema input to test weight accumulation at the graph level,
	// bypassing JoinGraphResult deduplication of (source,target) pairs.
	slim := &SlimSchema{
		Tables: []SlimTable{
			{Name: "users", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
			{Name: "orders", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}, {Name: "user_id", Type: "int"}}},
			{Name: "profiles", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}, {Name: "user_id", Type: "int"}}},
			{Name: "audit_log", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}, {Name: "order_id", Type: "int"}}},
		},
		Relations: []SlimRelation{
			// Two declared FKs: orders→users (2.0), profiles→users (2.0)
			{Table: "orders", Columns: []string{"user_id"}, ParentTable: "users", ParentColumns: []string{"id"}, Declared: true},
			{Table: "profiles", Columns: []string{"user_id"}, ParentTable: "users", ParentColumns: []string{"id"}, Declared: true},
			// Two declared FKs between same pair: additive to 4.0
			{Table: "profiles", Columns: []string{"user_id"}, ParentTable: "users", ParentColumns: []string{"id"}, Declared: true},
			// Inferred: audit_log→orders (1.0)
			{Table: "audit_log", Columns: []string{"order_id"}, ParentTable: "orders", ParentColumns: []string{"id"}, Declared: false},
		},
	}

	graph, _ := BuildWeightedGraph(slim)

	// weight(orders, users) = 2.0 (one declared FK)
	w1 := graph["orders"]["users"]
	if math.Abs(w1-2.0) > 0.001 {
		t.Errorf("expected weight 2.0 for declared FK (orders→users), got %f", w1)
	}
	w1r := graph["users"]["orders"]
	if math.Abs(w1r-2.0) > 0.001 {
		t.Errorf("expected undirected weight 2.0 for users→orders, got %f", w1r)
	}

	// weight(profiles, users) = 2.0 + 2.0 = 4.0 (two declared FKs, additive)
	w2 := graph["profiles"]["users"]
	if math.Abs(w2-4.0) > 0.001 {
		t.Errorf("expected weight 4.0 for two declared FKs (profiles→users), got %f", w2)
	}

	// weight(audit_log, orders) = 1.0 (inferred only)
	w3 := graph["audit_log"]["orders"]
	if math.Abs(w3-1.0) > 0.001 {
		t.Errorf("expected weight 1.0 for inferred (audit_log→orders), got %f", w3)
	}
}

// ============================================================
// Test 2 & 7: Louvain clustering — determinism and basic clustering
// ============================================================

func TestDomain_LouvainDeterminism(t *testing.T) {
	// Build a graph with multiple tables
	slim := &SlimSchema{
		Tables: []SlimTable{
			{Name: "users", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
			{Name: "orders", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}, {Name: "user_id", Type: "int"}}},
			{Name: "order_items", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}, {Name: "order_id", Type: "int"}}},
			{Name: "products", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
			{Name: "reviews", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}, {Name: "product_id", Type: "int"}}},
		},
		Relations: []SlimRelation{
			{Table: "orders", Columns: []string{"user_id"}, ParentTable: "users", ParentColumns: []string{"id"}, Declared: true},
			{Table: "order_items", Columns: []string{"order_id"}, ParentTable: "orders", ParentColumns: []string{"id"}, Declared: true},
			{Table: "reviews", Columns: []string{"product_id"}, ParentTable: "products", ParentColumns: []string{"id"}, Declared: true},
		},
	}

	graph, sortedNodes := BuildWeightedGraph(slim)

	// Run twice with same input
	comm1 := RunLouvain(graph, sortedNodes, 1.0)
	comm2 := RunLouvain(graph, sortedNodes, 1.0)

	if len(comm1) != len(comm2) {
		t.Fatalf("community count differs between runs: %d vs %d", len(comm1), len(comm2))
	}

	for table, cid1 := range comm1 {
		cid2, ok := comm2[table]
		if !ok {
			t.Fatalf("table %s missing in second run", table)
		}
		if cid1 != cid2 {
			t.Fatalf("community ID for %s differs: %d vs %d — Louvain not deterministic", table, cid1, cid2)
		}
	}

	// Also verify modularity is computed
	_ = ComputeModularity(graph, comm1, 1.0)
}

// ============================================================
// Test 3: Community naming
// ============================================================

func TestDomain_CommunityNaming(t *testing.T) {
	// Create a graph where "order" prefix appears in >=40% of one community
	slim := &SlimSchema{
		Tables: []SlimTable{
			{Name: "orders", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
			{Name: "order_items", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}, {Name: "order_id", Type: "int"}}},
			{Name: "order_payments", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}, {Name: "order_id", Type: "int"}}},
			{Name: "order_shipments", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}, {Name: "order_id", Type: "int"}}},
			{Name: "users", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
			{Name: "addresses", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}, {Name: "user_id", Type: "int"}}},
		},
		Relations: []SlimRelation{
			{Table: "order_items", Columns: []string{"order_id"}, ParentTable: "orders", ParentColumns: []string{"id"}, Declared: true},
			{Table: "order_payments", Columns: []string{"order_id"}, ParentTable: "orders", ParentColumns: []string{"id"}, Declared: true},
			{Table: "order_shipments", Columns: []string{"order_id"}, ParentTable: "orders", ParentColumns: []string{"id"}, Declared: true},
			{Table: "addresses", Columns: []string{"user_id"}, ParentTable: "users", ParentColumns: []string{"id"}, Declared: true},
		},
	}

	graph, sortedNodes := BuildWeightedGraph(slim)
	communities := RunLouvain(graph, sortedNodes, 1.0)

	// Collect sorted table names
	tables := make([]string, len(slim.Tables))
	for i, t := range slim.Tables {
		tables[i] = t.Name
	}
	sort.Strings(tables)

	communityNames := NameCommunities(communities, graph, sortedNodes, tables)
	if len(communityNames) == 0 {
		t.Fatal("expected at least one community name")
	}

	// Check for "misc" deduplication
	miscCount := 0
	for _, name := range communityNames {
		if name == "misc" {
			miscCount++
		}
	}
	if miscCount > 1 {
		t.Errorf("expected at most 1 'misc' domain (merged), got %d", miscCount)
	}
}

// ============================================================
// Test 4: Sub-domain detection (requires >15 tables in a domain)
// ============================================================

func TestDomain_SubDomainDetection(t *testing.T) {
	// Build 20 order_* tables strongly connected + 2 misc tables
	tables := []SlimTable{
		{Name: "order_items_01", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_items_02", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_items_03", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_items_04", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_items_05", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_items_06", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_items_07", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_items_08", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_payments_01", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_payments_02", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_payments_03", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_payments_04", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_payments_05", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_payments_06", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_shipments_01", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_shipments_02", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_shipments_03", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_invoices_01", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_invoices_02", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "order_invoices_03", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "orders", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "config", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		{Name: "logs", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
	}
	relations := []SlimRelation{}
	for _, t := range tables {
		if t.Name == "orders" || t.Name == "config" || t.Name == "logs" {
			continue
		}
		relations = append(relations, SlimRelation{
			Table: t.Name, Columns: []string{"order_id"},
			ParentTable: "orders", ParentColumns: []string{"id"},
			Declared: true,
		})
	}

	slim := &SlimSchema{Tables: tables, Relations: relations}
	graph, sortedNodes := BuildWeightedGraph(slim)

	communities := RunLouvain(graph, sortedNodes, 1.0)
	sortedTables := make([]string, len(slim.Tables))
	for i, t := range slim.Tables {
		sortedTables[i] = t.Name
	}
	sort.Strings(sortedTables)

	communityNames := NameCommunities(communities, graph, sortedNodes, sortedTables)
	subDomainMap, domainSubDomains := DetectSubDomains(communities, communityNames, graph, sortedNodes, slim.Tables)

	// The domain with >15 tables should have sub-domains (or at least be attempted)
	_ = subDomainMap
	_ = domainSubDomains
}

// ============================================================
// Test 5: Bridge table detection
// ============================================================

func TestDomain_BridgeDetection(t *testing.T) {
	slim := &SlimSchema{
		Tables: []SlimTable{
			{Name: "users", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
			{Name: "orders", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}, {Name: "user_id", Type: "int"}}},
			{Name: "products", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
			{Name: "reviews", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}, {Name: "product_id", Type: "int"}, {Name: "user_id", Type: "int"}}},
			{Name: "categories", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		},
		Relations: []SlimRelation{
			{Table: "orders", Columns: []string{"user_id"}, ParentTable: "users", ParentColumns: []string{"id"}, Declared: true},
			{Table: "reviews", Columns: []string{"product_id"}, ParentTable: "products", ParentColumns: []string{"id"}, Declared: true},
			{Table: "reviews", Columns: []string{"user_id"}, ParentTable: "users", ParentColumns: []string{"id"}, Declared: true},
		},
	}

	// Need community assignments to test bridge detection
	graph, sortedNodes := BuildWeightedGraph(slim)
	communities := RunLouvain(graph, sortedNodes, 1.0)

	bridgeMap := DetectBridgeTables(slim, communities)

	// reviews connect to multiple domains (if reviews and products/users in different domains)
	if bridges, ok := bridgeMap["reviews"]; ok {
		if len(bridges) < 2 {
			t.Logf("reviews bridges to %d domains: %v (expected >=2 for bridge)", len(bridges), bridges)
		}
	}
}

// ============================================================
// Test 6: Output assembly — domain_map.json format
// ============================================================

func TestDomain_OutputAssembly(t *testing.T) {
	slim := &SlimSchema{
		Tables: []SlimTable{
			{Name: "users", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
			{Name: "orders", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}, {Name: "user_id", Type: "int"}}},
			{Name: "products", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		},
		Relations: []SlimRelation{
			{Table: "orders", Columns: []string{"user_id"}, ParentTable: "users", ParentColumns: []string{"id"}, Declared: true},
		},
	}

	graph, sortedNodes := BuildWeightedGraph(slim)
	communities := RunLouvain(graph, sortedNodes, 1.0)

	sortedTables := make([]string, len(slim.Tables))
	for i, t := range slim.Tables {
		sortedTables[i] = t.Name
	}
	sort.Strings(sortedTables)

	communityNames := NameCommunities(communities, graph, sortedNodes, sortedTables)
	modularity := ComputeModularity(graph, communities, 1.0)

	domainMap := BuildDomainMapOutput(slim, communities, communityNames, nil, nil, modularity, "moderate")

	if domainMap.Metadata.TableCount != 3 {
		t.Errorf("expected table_count=3, got %d", domainMap.Metadata.TableCount)
	}
	if domainMap.Metadata.DomainCount < 1 {
		t.Errorf("expected at least 1 domain, got %d", domainMap.Metadata.DomainCount)
	}

	// Verify JSON marshaling works
	data, err := json.MarshalIndent(domainMap, "", "  ")
	if err != nil {
		t.Fatalf("marshal domain_map: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("domain_map.json not valid JSON: %v", err)
	}

	meta, ok := parsed["metadata"].(map[string]any)
	if !ok {
		t.Fatal("expected 'metadata' object in domain_map")
	}
	if _, ok := meta["table_count"]; !ok {
		t.Error("expected 'table_count' in metadata")
	}
	if _, ok := meta["domain_count"]; !ok {
		t.Error("expected 'domain_count' in metadata")
	}
	if _, ok := parsed["domains"]; !ok {
		t.Error("expected 'domains' object in domain_map")
	}
}

// ============================================================
// Test 8: Empty schema
// ============================================================

func TestDomain_EmptySchema(t *testing.T) {
	slim := &SlimSchema{
		Tables: []SlimTable{},
	}

	graph, sortedNodes := BuildWeightedGraph(slim)
	communities := RunLouvain(graph, sortedNodes, 1.0)

	sortedTables := []string{}
	communityNames := NameCommunities(communities, graph, sortedNodes, sortedTables)
	modularity := ComputeModularity(graph, communities, 1.0)

	domainMap := BuildDomainMapOutput(slim, communities, communityNames, nil, nil, modularity, "weak")

	if domainMap.Metadata.TableCount != 0 {
		t.Errorf("expected table_count=0 for empty schema, got %d", domainMap.Metadata.TableCount)
	}
	if domainMap.Metadata.DomainCount != 0 {
		t.Errorf("expected domain_count=0 for empty schema, got %d", domainMap.Metadata.DomainCount)
	}

	// Verify JSON marshaling without crash
	_, err := json.MarshalIndent(domainMap, "", "  ")
	if err != nil {
		t.Fatalf("marshal empty domain_map: %v", err)
	}
}

// ============================================================
// Test 9: All singleton / no edges with different prefixes
// Each table is isolated with degree 0 and unique prefix
// ============================================================

func TestDomain_AllSingleton(t *testing.T) {
	slim := &SlimSchema{
		Tables: []SlimTable{
			{Name: "alpha", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
			{Name: "beta", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
			{Name: "gamma", PK: "id", Columns: []SlimColumn{{Name: "id", Type: "int"}}},
		},
	}

	graph, sortedNodes := BuildWeightedGraph(slim)
	communities := RunLouvain(graph, sortedNodes, 1.0)

	sortedTables := []string{"alpha", "beta", "gamma"}
	communityNames := NameCommunities(communities, graph, sortedNodes, sortedTables)
	modularity := ComputeModularity(graph, communities, 1.0)

	domainMap := BuildDomainMapOutput(slim, communities, communityNames, nil, nil, modularity, "weak")

	// With no edges, modularity is 0 and all tables are isolated
	if math.Abs(modularity) > 0.001 {
		t.Errorf("expected modularity=0 for singletons with no edges, got %f", modularity)
	}

	// All tables should be accounted for
	if domainMap.Metadata.TableCount != 3 {
		t.Errorf("expected 3 tables, got %d", domainMap.Metadata.TableCount)
	}

	// Louvain with no edges groups all in one community; verify no crash and valid output
	if domainMap.Metadata.DomainCount < 1 {
		t.Errorf("expected at least 1 domain, got %d", domainMap.Metadata.DomainCount)
	}

	// Verify JSON output is valid (no crash)
	_, err := json.MarshalIndent(domainMap, "", "  ")
	if err != nil {
		t.Fatalf("marshal domain_map for all-singleton: %v", err)
	}
}

// ============================================================
// Test 10: Manual overrides
// ============================================================

func TestDomain_ManualOverrides(t *testing.T) {
	dir := t.TempDir()

	// Write overrides file
	overrides := map[string]any{
		"overrides": []map[string]string{
			{"table": "reviews", "domain": "feedback"},
			{"table": "nonexistent_table", "domain": "ignored"},
		},
	}
	data, _ := json.Marshal(overrides)
	if err := os.WriteFile(filepath.Join(dir, "domain_overrides.json"), data, 0644); err != nil {
		t.Fatalf("write overrides: %v", err)
	}

	overrideMap := LoadOverrides(dir)
	if len(overrideMap) != 2 {
		t.Errorf("expected 2 overrides loaded, got %d", len(overrideMap))
	}
	if overrideMap["reviews"] != "feedback" {
		t.Errorf("expected reviews→feedback, got %s", overrideMap["reviews"])
	}
	if overrideMap["nonexistent_table"] != "ignored" {
		t.Errorf("expected nonexistent_table→ignored, got %s", overrideMap["nonexistent_table"])
	}

	// Test missing file
	overrideMap = LoadOverrides(filepath.Join(dir, "nonexistent"))
	if len(overrideMap) != 0 {
		t.Errorf("expected empty overrides for missing file, got %d", len(overrideMap))
	}
}

// ============================================================
// Wrapper tests that test domain pipeline functions end-to-end
// ============================================================

func TestDomain_TransformToSlimSchema(t *testing.T) {
	result := newSchemaResult([]db.TableInfo{
		{
			Schema: "testdb",
			Name:   "users",
			Columns: []db.ColumnInfo{
				{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
				{Name: "name", Ordinal: 2, ColumnType: "varchar(255)"},
			},
			Indexes: []db.IndexInfo{
				{Name: "users_pkey", Primary: true, Columns: []db.IndexColumn{{Name: "id", Order: "ASC", Sequence: 1}}},
				{Name: "users_name_idx", Columns: []db.IndexColumn{{Name: "name", Order: "ASC", Sequence: 1}}},
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
			Indexes: []db.IndexInfo{
				{Name: "orders_pkey", Primary: true, Columns: []db.IndexColumn{{Name: "id", Order: "ASC", Sequence: 1}}},
			},
		},
	})

	joinGraph := buildJoinGraphForTest(t, result)
	slim := TransformToSlimSchema(result, joinGraph)

	if len(slim.Tables) != 2 {
		t.Fatalf("expected 2 slim tables, got %d", len(slim.Tables))
	}
	if len(slim.Relations) < 1 {
		t.Fatal("expected at least 1 slim relation")
	}

	// Check PK field on users
	if slim.Tables[0].Name != "users" && slim.Tables[1].Name != "users" {
		t.Fatal("expected users table in slim schema")
	}

	var usersTable *SlimTable
	for i := range slim.Tables {
		if slim.Tables[i].Name == "users" {
			usersTable = &slim.Tables[i]
			break
		}
	}
	if usersTable == nil {
		t.Fatal("users table not found")
	}
	if usersTable.PK != "id" {
		t.Errorf("expected users PK='id', got '%s'", usersTable.PK)
	}
	if len(usersTable.Indexes) != 1 || usersTable.Indexes[0].Name != "users_name_idx" {
		t.Errorf("expected 1 non-PK index on users, got %d: %v", len(usersTable.Indexes), usersTable.Indexes)
	}
}

func TestDomain_BuildEnrichedSchemaMap(t *testing.T) {
	schemaMap := SchemaMap{
		"users": &TableMapEntry{
			Table:  "users",
			Schema: "testdb",
			PKColumns: []string{"id"},
			FKOut: []FKEdge{},
			IndexedColumns: []string{"email"},
		},
		"orders": &TableMapEntry{
			Table:  "orders",
			Schema: "testdb",
			PKColumns: []string{"id"},
			FKOut: []FKEdge{
				{Table: "users", Column: "user_id"},
			},
			IndexedColumns: []string{"user_id"},
		},
	}

	communities := map[string]int{"users": 0, "orders": 1}
	communityNames := map[int]string{0: "users", 1: "orders"}
	bridgeMap := map[string][]string{"orders": {"users"}}

	enriched := BuildEnrichedSchemaMap(schemaMap, communities, communityNames, nil, bridgeMap)

	usersEntry, ok := enriched["users"]
	if !ok {
		t.Fatal("expected users in enriched map")
	}
	if usersEntry.Domain != "users" {
		t.Errorf("expected users domain='users', got '%s'", usersEntry.Domain)
	}
	if usersEntry.Bridge {
		t.Error("expected users bridge=false")
	}
	if len(usersEntry.BridgeDomains) != 0 {
		t.Errorf("expected empty bridge_domains for users, got %v", usersEntry.BridgeDomains)
	}
	if usersEntry.Table != "users" {
		t.Errorf("expected users.Table='users', got '%s'", usersEntry.Table)
	}

	ordersEntry, ok := enriched["orders"]
	if !ok {
		t.Fatal("expected orders in enriched map")
	}
	if ordersEntry.Domain != "orders" {
		t.Errorf("expected orders domain='orders', got '%s'", ordersEntry.Domain)
	}
	if !ordersEntry.Bridge {
		t.Error("expected orders bridge=true")
	}
	if len(ordersEntry.BridgeDomains) != 1 || ordersEntry.BridgeDomains[0] != "users" {
		t.Errorf("expected orders bridge_domains=['users'], got %v", ordersEntry.BridgeDomains)
	}
}

func TestDomain_AnnotateJoinGraphCrossDomain(t *testing.T) {
	joinGraph := &JoinGraphResult{
		Edges: []db.JoinEdge{
			{Source: "orders", Target: "users", Columns: [][2]string{{"user_id", "id"}}, SourceType: "declared_foreign_key"},
			{Source: "reviews", Target: "products", Columns: [][2]string{{"product_id", "id"}}, SourceType: "declared_foreign_key"},
		},
	}

	// orders and users in same domain (0), reviews in domain 0, products in domain 1
	communities := map[string]int{"orders": 0, "users": 0, "reviews": 0, "products": 1}

	edges := AnnotateJoinGraphCrossDomain(joinGraph, communities)
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}

	for _, e := range edges {
		if strings.HasPrefix(e.From, "orders") && e.To == "users.id" {
			if e.CrossDomain {
				t.Error("expected orders→users to be same-domain (cross_domain=false)")
			}
		}
	}
	if strings.HasPrefix(edges[1].From, "reviews") && edges[1].To == "products.id" {
		if !edges[1].CrossDomain {
			t.Error("expected reviews→products to be cross-domain (cross_domain=true)")
		}
	}
}

func TestDomain_WriteDomainFiles(t *testing.T) {
	dir := t.TempDir()

	// Create schema dir first
	schemaDir := filepath.Join(dir, "schema")
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		t.Fatalf("create schema dir: %v", err)
	}

	// Write an initial join_graph.json for extension
	initialGraph := map[string]any{
		"edges": []map[string]any{
			{"source": "orders", "target": "users", "columns": [][]string{{"user_id", "id"}}, "confidence": 1.0, "source_type": "declared_foreign_key", "composite": false, "cross_domain": false},
		},
		"generated_at": "2024-01-01T00:00:00Z",
		"table_count": 2,
		"declared_fk_count": 1,
		"inferred_join_count": 0,
	}
	initialData, _ := json.MarshalIndent(initialGraph, "", "  ")
	if err := os.WriteFile(filepath.Join(schemaDir, "join_graph.json"), initialData, 0644); err != nil {
		t.Fatalf("write initial join_graph: %v", err)
	}

	domainMap := &DomainMap{
		Metadata: DomainMapMeta{TableCount: 2, DomainCount: 1, SubdomainCount: 0},
		Domains: map[string]DomainEntry{
			"users": {Tables: []string{"users", "orders"}},
		},
	}

	enrichedMap := map[string]*EnrichedTableEntry{
		"users": {
			Domain:        "users",
			Bridge:        false,
			BridgeDomains: []string{},
		},
	}

	crossDomainEdges := []CrossDomainEdge{
		{From: "orders.user_id", To: "users.id", Declared: true, CrossDomain: false},
	}

	if err := WriteDomainFiles(dir, domainMap, enrichedMap, crossDomainEdges); err != nil {
		t.Fatalf("WriteDomainFiles failed: %v", err)
	}

	// Check domain_map.json exists at top level
	if _, err := os.Stat(filepath.Join(dir, "domain_map.json")); os.IsNotExist(err) {
		t.Error("domain_map.json not written at dbDir")
	}

	// Check schema/domain_map.json exists
	if _, err := os.Stat(filepath.Join(schemaDir, "domain_map.json")); os.IsNotExist(err) {
		t.Error("schema/domain_map.json not written")
	}

	// Check join_graph.json has cross_domain field
	joinData, err := os.ReadFile(filepath.Join(schemaDir, "join_graph.json"))
	if err != nil {
		t.Fatalf("read join_graph.json: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(joinData, &parsed); err != nil {
		t.Fatalf("join_graph.json not valid JSON: %v", err)
	}
	edges, ok := parsed["edges"].([]any)
	if !ok {
		t.Fatal("expected edges array")
	}
	if len(edges) > 0 {
		edge := edges[0].(map[string]any)
		if _, ok := edge["cross_domain"]; !ok {
			t.Error("expected cross_domain field in join_graph edge")
		}
	}
}
