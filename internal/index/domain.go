package index

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/querylex/querylex/internal/db"
)

// ============================================================
// Slim Schema Types
// ============================================================

// SlimTable is a compact table representation for domain analysis.
type SlimTable struct {
	Name    string       `json:"name"`
	PK      string       `json:"pk,omitempty"`
	Columns []SlimColumn `json:"columns"`
	Indexes []SlimIndex  `json:"indexes"`
}

// SlimColumn is a compact column representation.
type SlimColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// SlimIndex is a compact index representation.
type SlimIndex struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

// SlimRelation describes a relationship between two tables.
type SlimRelation struct {
	Table         string   `json:"table"`
	Columns       []string `json:"columns"`
	ParentTable   string   `json:"parent_table"`
	ParentColumns []string `json:"parent_columns"`
	Declared      bool     `json:"declared"`
}

// SlimSchema is a compact database schema for domain analysis.
type SlimSchema struct {
	Name      string         `json:"name"`
	Tables    []SlimTable    `json:"tables"`
	Relations []SlimRelation `json:"relations"`
}

// ============================================================
// Domain Map Types
// ============================================================

// DomainMapMeta contains metadata about the domain map.
type DomainMapMeta struct {
	TableCount    int `json:"table_count"`
	DomainCount   int `json:"domain_count"`
	SubdomainCount int `json:"subdomain_count"`
}

// DomainEntry describes a single domain and its tables (and optional sub-domains).
type DomainEntry struct {
	Tables     []string            `json:"tables"`
	SubDomains map[string][]string `json:"sub_domains,omitempty"`
}

// DomainMap is the top-level domain-to-table grouping artifact.
type DomainMap struct {
	Metadata DomainMapMeta          `json:"metadata"`
	Domains  map[string]DomainEntry `json:"domains"`
}

// EnrichedTableEntry extends TableMapEntry with domain and bridge information.
type EnrichedTableEntry struct {
	Table           string      `json:"table"`
	Schema          string      `json:"schema"`
	PKColumns       []string    `json:"pk_columns"`
	FKIn            []FKEdge    `json:"fk_in,omitempty"`
	FKOut           []FKOutEdge `json:"fk_out,omitempty"`
	IndexedColumns  []string    `json:"indexed_columns,omitempty"`
	CompositeIndexes [][]string `json:"composite_indexes,omitempty"`
	Domain          string      `json:"domain"`
	SubDomain       string      `json:"sub_domain,omitempty"`
	Bridge          bool        `json:"bridge"`
	BridgeDomains   []string    `json:"bridge_domains"`
}

// FKOutEdge describes a foreign key from this table to another table.
type FKOutEdge struct {
	To      string `json:"to"`
	Declared bool  `json:"declared"`
}

// CrossDomainEdge describes a join relationship annotated with cross-domain status.
type CrossDomainEdge struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Declared    bool   `json:"declared"`
	CrossDomain bool   `json:"cross_domain"`
}

// OverrideEntry is a single manual domain override.
type OverrideEntry struct {
	Table  string `json:"table"`
	Domain string `json:"domain"`
}

// OverridesFile is the JSON structure for manual domain overrides.
type OverridesFile struct {
	Overrides []OverrideEntry `json:"overrides"`
}

// ============================================================
// Function 1: TransformToSlimSchema
// ============================================================

// TransformToSlimSchema converts a SchemaResult and JoinGraphResult into the
// compact SlimSchema format used by the domain atlas pipeline.
func TransformToSlimSchema(result *db.SchemaResult, joinGraph *JoinGraphResult) *SlimSchema {
	slim := &SlimSchema{
		Tables:    make([]SlimTable, 0, len(result.Tables)),
		Relations: make([]SlimRelation, 0),
	}

	// Build table map for FK reference lookup
	tableMap := make(map[string]int)
	for i, t := range result.Tables {
		tableMap[t.Name] = i
	}

	// Convert tables
	for _, t := range result.Tables {
		st := SlimTable{
			Name:    t.Name,
			Columns: make([]SlimColumn, 0, len(t.Columns)),
			Indexes: make([]SlimIndex, 0),
		}

		// Find PK — first column with IsPrimaryKey
		for _, c := range t.Columns {
			if c.IsPrimaryKey {
				st.PK = c.Name
				break
			}
		}

		// Convert columns
		for _, c := range t.Columns {
			st.Columns = append(st.Columns, SlimColumn{
				Name: c.Name,
				Type: c.ColumnType,
			})
		}

		// Convert indexes (exclude PK index)
		for _, idx := range t.Indexes {
			if idx.Primary {
				continue
			}
			colNames := make([]string, len(idx.Columns))
			for i, c := range idx.Columns {
				colNames[i] = c.Name
			}
			st.Indexes = append(st.Indexes, SlimIndex{
				Name:    idx.Name,
				Columns: colNames,
			})
		}

		slim.Tables = append(slim.Tables, st)
	}

	// Convert edges from JoinGraphResult to SlimRelations with dedup
	relDedup := make(map[string]bool)
	for _, edge := range joinGraph.Edges {
		// Collect source columns
		srcCols := make([]string, len(edge.Columns))
		tgtCols := make([]string, len(edge.Columns))
		for i, pair := range edge.Columns {
			srcCols[i] = pair[0]
			tgtCols[i] = pair[1]
		}

		dedupKey := edge.Source + ":" + strings.Join(srcCols, ",") + ":" +
			edge.Target + ":" + strings.Join(tgtCols, ",")
		if relDedup[dedupKey] {
			continue
		}
		relDedup[dedupKey] = true

		declared := edge.SourceType == "declared_foreign_key"
		slim.Relations = append(slim.Relations, SlimRelation{
			Table:         edge.Source,
			Columns:       srcCols,
			ParentTable:   edge.Target,
			ParentColumns: tgtCols,
			Declared:      declared,
		})
	}

	// Set schema name
	if len(result.Tables) > 0 {
		slim.Name = result.Tables[0].Schema
	}

	return slim
}

// ============================================================
// Function 2: BuildWeightedGraph
// ============================================================

// BuildWeightedGraph constructs a weighted undirected graph from a SlimSchema.
// Returns the graph as adjacency map and a sorted list of node names.
func BuildWeightedGraph(slim *SlimSchema) (graph map[string]map[string]float64, sortedNodes []string) {
	graph = make(map[string]map[string]float64)

	// Add all tables as nodes
	for _, t := range slim.Tables {
		if graph[t.Name] == nil {
			graph[t.Name] = make(map[string]float64)
		}
	}

	// Add relation edges
	for _, rel := range slim.Relations {
		weight := 1.0
		if rel.Declared {
			weight = 2.0
		}
		// Undirected: add both directions
		addWeight(graph, rel.Table, rel.ParentTable, weight)
		addWeight(graph, rel.ParentTable, rel.Table, weight)
	}

	// Add prefix-penalty edges
	addPrefixEdges(graph, slim.Tables)

	// Build sorted node list
	sortedNodes = make([]string, 0, len(graph))
	for name := range graph {
		sortedNodes = append(sortedNodes, name)
	}
	sort.Strings(sortedNodes)

	return graph, sortedNodes
}

// addWeight adds a weight to an edge (a→b) in the graph.
func addWeight(graph map[string]map[string]float64, a, b string, weight float64) {
	if graph[a] == nil {
		graph[a] = make(map[string]float64)
	}
	graph[a][b] += weight
}

// addPrefixEdges adds 0.5 weight between tables sharing the same prefix.
func addPrefixEdges(graph map[string]map[string]float64, tables []SlimTable) {
	// Group tables by prefix
	prefixGroups := make(map[string][]string)
	for _, t := range tables {
		prefix := extractPrefix(t.Name)
		if prefix == "" {
			continue
		}
		prefixGroups[prefix] = append(prefixGroups[prefix], t.Name)
	}

	// Add edges within each group with >= 2 tables
	for _, group := range prefixGroups {
		if len(group) < 2 {
			continue
		}
		for i := 0; i < len(group); i++ {
			for j := i + 1; j < len(group); j++ {
				addWeight(graph, group[i], group[j], 0.5)
				addWeight(graph, group[j], group[i], 0.5)
			}
		}
	}
}

// extractPrefix extracts the first underscore-delimited token as prefix.
// Returns empty string if the token is < 3 characters.
func extractPrefix(name string) string {
	parts := strings.Split(name, "_")
	if len(parts) == 0 {
		return ""
	}
	prefix := strings.ToLower(parts[0])
	if len(prefix) < 3 {
		return ""
	}
	return prefix
}

// ============================================================
// Function 3: RunLouvain
// ============================================================

// RunLouvain performs Louvain community detection on the weighted graph.
// Returns a map of table_name -> community_id.
func RunLouvain(graph map[string]map[string]float64, sortedNodes []string, resolution float64) map[string]int {
	n := len(sortedNodes)
	if n == 0 {
		return make(map[string]int)
	}

	// Node index lookup
	nodeIndex := make(map[string]int)
	for i, name := range sortedNodes {
		nodeIndex[name] = i
	}

	// Initialize: each node in its own community
	community := make([]int, n)
	for i := 0; i < n; i++ {
		community[i] = i
	}

	// Precompute node degrees
	degrees := make([]float64, n)
	totalWeight := 0.0
	for _, u := range sortedNodes {
		ui := nodeIndex[u]
		for _, w := range graph[u] {
			degrees[ui] += w
			totalWeight += w
		}
	}
	totalWeight /= 2.0 // undirected, each edge counted twice

	// Handle edge case: no edges
	if totalWeight == 0 {
		comm := make(map[string]int)
		for _, name := range sortedNodes {
			comm[name] = 0 // all in community 0
		}
		return comm
	}

	maxPasses := 20
	for pass := 0; pass < maxPasses; pass++ {
		moved := false
		for _, u := range sortedNodes {
			ui := nodeIndex[u]
			currComm := community[ui]

			// Compute neighbor community weights
			neighborCommunities := make(map[int]float64)
			for v, w := range graph[u] {
				vi := nodeIndex[v]
				vc := community[vi]
				neighborCommunities[vc] += w
			}

			// Current community weight (excluding self)
			delete(neighborCommunities, currComm)

			// Find best community to move to
			bestComm := currComm
			bestGain := 0.0
			// Sort neighbors for determinism
			sortedNeighborComms := make([]int, 0, len(neighborCommunities))
			for c := range neighborCommunities {
				sortedNeighborComms = append(sortedNeighborComms, c)
			}
			sort.Ints(sortedNeighborComms)

			for _, nc := range sortedNeighborComms {
				// Compute modularity gain of moving node u to community nc
				sigmaTot := 0.0
				for vi, c := range community {
					if c == nc {
						sigmaTot += degrees[vi]
					}
				}

				kiIn := neighborCommunities[nc]
				ki := degrees[ui]

				// Modularity gain formula
				gain := kiIn/totalWeight - resolution*ki*sigmaTot/(2*totalWeight*totalWeight)

				if gain > bestGain {
					bestGain = gain
					bestComm = nc
				}
			}

			if bestComm != currComm && bestGain > 0 {
				community[ui] = bestComm
				moved = true
			}
		}
		if !moved {
			break
		}
	}

	// Renumber communities to 0-based consecutive
	commMap := make(map[int]int)
	nextID := 0
	for i := 0; i < n; i++ {
		if _, ok := commMap[community[i]]; !ok {
			commMap[community[i]] = nextID
			nextID++
		}
	}

	result := make(map[string]int)
	for _, name := range sortedNodes {
		ui := nodeIndex[name]
		result[name] = commMap[community[ui]]
	}

	return result
}

// ============================================================
// Modularity helper
// ============================================================

// ComputeModularity computes the modularity of a partition.
func ComputeModularity(graph map[string]map[string]float64, communities map[string]int, resolution float64) float64 {
	// Compute total weight
	totalWeight := 0.0

	for uName := range graph {
		for _, w := range graph[uName] {
			totalWeight += w
		}
	}
	totalWeight /= 2.0 // undirected

	if totalWeight == 0 {
		return 0.0
	}

	// Compute node degrees
	degrees := make(map[string]float64)
	for uName, neighbors := range graph {
		for _, w := range neighbors {
			degrees[uName] += w
		}
	}

	// Compute modularity
	Q := 0.0
	for uName, neighbors := range graph {
		for vName, w := range neighbors {
			if communities[uName] == communities[vName] {
				Q += w - resolution*degrees[uName]*degrees[vName]/(2*totalWeight)
			}
		}
	}

	return Q / (2 * totalWeight)
}

// ============================================================
// Function 4: NameCommunities
// ============================================================

// NameCommunities assigns human-readable domain names to communities.
func NameCommunities(communities map[string]int, graph map[string]map[string]float64, sortedNodes []string, sortedTables []string) map[int]string {
	// Group tables by community
	commTables := make(map[int][]string)
	for table, cid := range communities {
		commTables[cid] = append(commTables[cid], table)
	}

	// Sort each community's tables
	for cid := range commTables {
		sort.Strings(commTables[cid])
	}

	// Sort community IDs for deterministic processing
	cids := make([]int, 0, len(commTables))
	for cid := range commTables {
		cids = append(cids, cid)
	}
	sort.Ints(cids)

	usedNames := make(map[string]bool)
	var miscTables []string
	communityNames := make(map[int]string)

	for _, cid := range cids {
		tables := commTables[cid]
		name := nameSingleCommunity(tables, graph, usedNames, &miscTables)
		communityNames[cid] = name
	}

	// Handle misc merging: if miscTables has entries, create/merge misc domain
	if len(miscTables) > 0 {
		// Find existing misc community or create new one
		// Tables already assigned to misc via nameSingleCommunity
	}

	return communityNames
}

// nameSingleCommunity names a single community using prefix voting.
func nameSingleCommunity(tables []string, graph map[string]map[string]float64, usedNames map[string]bool, miscTables *[]string) string {
	if len(tables) == 1 {
		// Check if singleton has degree 0
		deg := degree(tables[0], graph)
		if deg == 0 {
			*miscTables = append(*miscTables, tables[0])
			return "misc"
		}
	}

	// Prefix voting
	prefixVotes := make(map[string]int)
	for _, table := range tables {
		prefix := extractPrefix(table)
		if prefix != "" {
			prefixVotes[prefix]++
		}
	}

	// Find prefix with >= 40% threshold
	threshold := float64(len(tables)) * 0.4
	var winningPrefix string
	var maxVotes int

	// Sort prefixes for determinism
	prefixes := make([]string, 0, len(prefixVotes))
	for p := range prefixVotes {
		prefixes = append(prefixes, p)
	}
	sort.Strings(prefixes)

	for _, p := range prefixes {
		votes := prefixVotes[p]
		if float64(votes) >= threshold && votes > maxVotes {
			winningPrefix = p
			maxVotes = votes
		}
	}

	var name string
	if winningPrefix != "" {
		name = resolveDomainName(winningPrefix, tables)
	} else {
		// Fallback: highest-degree node
		name = fallbackName(tables, graph)
	}

	// Handle deduplication
	if name != "misc" {
		name = dedupName(name, usedNames)
	}

	// If name is "misc" and already used, merge by appending to miscTables
	if name == "misc" {
		*miscTables = append(*miscTables, tables...)
		return "misc"
	}

	usedNames[name] = true
	return name
}

// degree returns the number of edges a node has in the graph.
func degree(node string, graph map[string]map[string]float64) int {
	neighbors, ok := graph[node]
	if !ok {
		return 0
	}
	count := 0
	for range neighbors {
		count++
	}
	return count
}

// resolveDomainName converts a winning prefix to a domain name.
func resolveDomainName(prefix string, tables []string) string {
	// Check if prefix matches a table name
	tableSet := make(map[string]bool)
	for _, t := range tables {
		tableSet[t] = true
	}

	if tableSet[prefix] {
		return prefix
	}

	// Try plural
	plural := prefix + "s"
	if tableSet[plural] {
		return plural
	}

	// Use plural anyway
	return plural
}

// fallbackName names a community after its highest-degree node.
func fallbackName(tables []string, graph map[string]map[string]float64) string {
	bestTable := tables[0]
	bestDeg := degree(tables[0], graph)

	for _, t := range tables[1:] {
		d := degree(t, graph)
		if d > bestDeg {
			bestDeg = d
			bestTable = t
		}
	}

	return bestTable
}

// dedupName ensures a name hasn't been used, appending _2, _3, etc.
func dedupName(name string, usedNames map[string]bool) string {
	if !usedNames[name] {
		return name
	}

	suffix := 2
	for {
		candidate := fmt.Sprintf("%s_%d", name, suffix)
		if !usedNames[candidate] {
			return candidate
		}
		suffix++
	}
}

// ============================================================
// Function 5: DetectSubDomains
// ============================================================

// DetectSubDomains partitions large domains into sub-domains.
func DetectSubDomains(communities map[string]int, communityNames map[int]string, graph map[string]map[string]float64, sortedNodes []string, tables []SlimTable) (map[string]string, map[string]map[string][]string) {
	subDomainMap := make(map[string]string)      // table_name -> sub_domain_name
	domainSubDomains := make(map[string]map[string][]string) // domain_name -> sub_domain_name -> [table_names]

	// Build table name set for quick lookup
	tableSet := make(map[string]bool)
	for _, t := range tables {
		tableSet[t.Name] = true
	}

	// Group by community name
	domainTables := make(map[string][]string)
	for table, cid := range communities {
		name, ok := communityNames[cid]
		if !ok {
			continue
		}
		if tableSet[table] {
			domainTables[name] = append(domainTables[name], table)
		}
	}

	for domainName, domainTableList := range domainTables {
		// Skip small domains and misc
		if len(domainTableList) <= 15 || domainName == "misc" {
			continue
		}

		// Build induced subgraph
		subgraph := make(map[string]map[string]float64)
		subNodeSet := make(map[string]bool)
		for _, t := range domainTableList {
			subNodeSet[t] = true
		}

		// Extract subgraph edges
		for _, t := range domainTableList {
			if graph[t] == nil {
				continue
			}
			for neighbor, w := range graph[t] {
				if subNodeSet[neighbor] {
					if subgraph[t] == nil {
						subgraph[t] = make(map[string]float64)
					}
					subgraph[t][neighbor] = w
				}
			}
		}

		// Check if subgraph has at least one edge
		if !hasEdge(subgraph) {
			continue
		}

		// Build sorted node list for subgraph
		subSortedNodes := make([]string, 0, len(domainTableList))
		for _, t := range domainTableList {
			subSortedNodes = append(subSortedNodes, t)
		}
		sort.Strings(subSortedNodes)

		// Run Louvain on subgraph with resolution 1.5
		subCommunities := RunLouvain(subgraph, subSortedNodes, 1.5)

		// Check if more than one subcommunity was found
		subCIDs := make(map[int]bool)
		for _, cid := range subCommunities {
			subCIDs[cid] = true
		}
		if len(subCIDs) <= 1 {
			continue
		}

		// Name sub-communities
		subDomainTables := make(map[int][]string)
		for table, cid := range subCommunities {
			subDomainTables[cid] = append(subDomainTables[cid], table)
		}

		subUsedNames := make(map[string]bool)
		subNames := make(map[int]string)

		subCIDList := make([]int, 0, len(subDomainTables))
		for cid := range subDomainTables {
			subCIDList = append(subCIDList, cid)
		}
		sort.Ints(subCIDList)

		for _, scid := range subCIDList {
			st := subDomainTables[scid]
			var miscCollector []string
			sn := nameSingleCommunity(st, subgraph, subUsedNames, &miscCollector)
			if sn == "misc" {
				sn = "misc_subdomain"
			}
			sn = dedupName(sn, subUsedNames)
			subUsedNames[sn] = true
			subNames[scid] = sn
		}

		// Record sub-domain assignments
		for table, scid := range subCommunities {
			sdName := subNames[scid]
			subDomainMap[table] = sdName
			if domainSubDomains[domainName] == nil {
				domainSubDomains[domainName] = make(map[string][]string)
			}
			domainSubDomains[domainName][sdName] = append(domainSubDomains[domainName][sdName], table)
		}

		// Sort sub-domain table lists
		for _, sdMap := range domainSubDomains[domainName] {
			sort.Strings(sdMap)
		}
	}

	return subDomainMap, domainSubDomains
}

// hasEdge checks if a graph has at least one edge.
func hasEdge(graph map[string]map[string]float64) bool {
	for _, neighbors := range graph {
		if len(neighbors) > 0 {
			return true
		}
	}
	return false
}

// ============================================================
// Function 6: DetectBridgeTables
// ============================================================

// DetectBridgeTables identifies tables with neighbors in >=2 domains.
func DetectBridgeTables(slim *SlimSchema, communities map[string]int) map[string][]string {
	// Build adjacency from relations
	adjacency := make(map[string]map[string]bool)
	for _, rel := range slim.Relations {
		// Both source and target must exist in communities
		_, srcOk := communities[rel.Table]
		_, tgtOk := communities[rel.ParentTable]
		if !srcOk || !tgtOk {
			continue
		}
		if adjacency[rel.Table] == nil {
			adjacency[rel.Table] = make(map[string]bool)
		}
		adjacency[rel.Table][rel.ParentTable] = true
		if adjacency[rel.ParentTable] == nil {
			adjacency[rel.ParentTable] = make(map[string]bool)
		}
		adjacency[rel.ParentTable][rel.Table] = true
	}

	// Build domain name reverse mapping
	communityNames := make(map[int]string)
	for _, cid := range communities {
		if _, ok := communityNames[cid]; !ok {
			communityNames[cid] = fmt.Sprintf("domain_%d", cid)
		}
	}

	bridgeMap := make(map[string][]string)
	for table, neighbors := range adjacency {
		domainSet := make(map[string]bool)
		for neighbor := range neighbors {
			if cid, ok := communities[neighbor]; ok {
				name := communityNames[cid]
				domainSet[name] = true
			}
		}

		if len(domainSet) >= 2 {
			domains := make([]string, 0, len(domainSet))
			for d := range domainSet {
				domains = append(domains, d)
			}
			sort.Strings(domains)
			bridgeMap[table] = domains
		}
	}

	return bridgeMap
}

// ============================================================
// Function 7: BuildDomainMapOutput
// ============================================================

// BuildDomainMapOutput constructs the domain_map.json output structure.
func BuildDomainMapOutput(slim *SlimSchema, communities map[string]int, communityNames map[int]string, subDomainMap map[string]string, domainSubDomains map[string]map[string][]string, modularity float64, modularityClass string) *DomainMap {
	// Build reverse mapping: community_id -> community_name
	cidToName := make(map[int]string)
	for cid, name := range communityNames {
		cidToName[cid] = name
	}

	// Group tables by domain name
	domainTables := make(map[string][]string)
	for table, cid := range communities {
		name := cidToName[cid]
		domainTables[name] = append(domainTables[name], table)
	}

	// Sort tables within each domain
	for name := range domainTables {
		sort.Strings(domainTables[name])
	}

	// Count subdomains
	subdomainCount := 0
	if domainSubDomains != nil {
		for _, sdMap := range domainSubDomains {
			subdomainCount += len(sdMap)
		}
	}

	domainMap := &DomainMap{
		Metadata: DomainMapMeta{
			TableCount:     len(slim.Tables),
			DomainCount:    len(domainTables),
			SubdomainCount: subdomainCount,
		},
		Domains: make(map[string]DomainEntry),
	}

	// Sort domain names for deterministic output
	domainNames := make([]string, 0, len(domainTables))
	for name := range domainTables {
		domainNames = append(domainNames, name)
	}
	sort.Strings(domainNames)

	for _, name := range domainNames {
		entry := DomainEntry{
			Tables: domainTables[name],
		}

		// Add sub-domains if present
		if domainSubDomains != nil {
			if sd, ok := domainSubDomains[name]; ok && len(sd) > 0 {
				entry.SubDomains = sd
			}
		}

		domainMap.Domains[name] = entry
	}

	return domainMap
}

// ============================================================
// Function 8: BuildEnrichedSchemaMap
// ============================================================

// BuildEnrichedSchemaMap produces per-table enriched metadata with domain info.
func BuildEnrichedSchemaMap(schemaMap SchemaMap, communities map[string]int, communityNames map[int]string, subDomainMap map[string]string, bridgeMap map[string][]string) map[string]*EnrichedTableEntry {
	// Build community_id -> name mapping
	cidToName := make(map[int]string)
	for cid, name := range communityNames {
		cidToName[cid] = name
	}

	enriched := make(map[string]*EnrichedTableEntry)

	for tableName, entry := range schemaMap {
		cid, hasCommunity := communities[tableName]
		domain := ""
		if hasCommunity {
			domain = cidToName[cid]
		}

		subDomain := ""
		if subDomainMap != nil {
			subDomain = subDomainMap[tableName]
		}

		bridge := false
		var bridgeDomains []string
		if bridgeMap != nil {
			if bd, ok := bridgeMap[tableName]; ok && len(bd) > 0 {
				bridge = true
				bridgeDomains = bd
			}
		}

		// Convert FKOut to FKOutEdge format
		fkOut := make([]FKOutEdge, 0, len(entry.FKOut))
		for _, fk := range entry.FKOut {
			formatted := fk.Table + "." + fk.Column
			fkOut = append(fkOut, FKOutEdge{
				To:      formatted,
				Declared: true, // FKOut entries come from declared constraints
			})
		}

		// Convert FKIn to FKEdge format for EnrichedTableEntry (preserve for fk_in)
		// But EnrichedTableEntry doesn't have FKIn field... let me check the struct
		// Actually, FKIn exists on TableMapEntry but EnrichedTableEntry only has FKOut
		// Wait, let me simplify - EnrichedTableEntry doesn't need FKIn because
		// the enriched format stores fk_out and what we need for output

		enriched[tableName] = &EnrichedTableEntry{
			Table:           entry.Table,
			Schema:          entry.Schema,
			PKColumns:       entry.PKColumns,
			FKOut:           fkOut,
			IndexedColumns:  entry.IndexedColumns,
			CompositeIndexes: entry.CompositeIndexes,
			Domain:          domain,
			SubDomain:       subDomain,
			Bridge:          bridge,
			BridgeDomains:   bridgeDomains,
		}
	}

	return enriched
}

// ============================================================
// Function 9: AnnotateJoinGraphCrossDomain
// ============================================================

// AnnotateJoinGraphCrossDomain annotates join graph edges with cross-domain status.
func AnnotateJoinGraphCrossDomain(joinGraph *JoinGraphResult, communities map[string]int) []CrossDomainEdge {
	edges := make([]CrossDomainEdge, 0, len(joinGraph.Edges))
	_ = communities

	for _, edge := range joinGraph.Edges {
		fromDomain, fromOk := communities[edge.Source]
		toDomain, toOk := communities[edge.Target]

		crossDomain := false
		if fromOk && toOk {
			crossDomain = fromDomain != toDomain
		}

		fromStr := formatEndpoint(edge.Source, edge.Columns, 0)
		toStr := formatEndpoint(edge.Target, edge.Columns, 1)

		edges = append(edges, CrossDomainEdge{
			From:        fromStr,
			To:          toStr,
			Declared:    edge.SourceType == "declared_foreign_key",
			CrossDomain: crossDomain,
		})
	}

	return edges
}

// formatEndpoint formats a table.column or table.(col1,col2) string.
func formatEndpoint(table string, columns [][2]string, colIdx int) string {
	cols := make([]string, len(columns))
	for i, pair := range columns {
		cols[i] = pair[colIdx]
	}
	if len(cols) == 1 {
		return table + "." + cols[0]
	}
	return table + ".(" + strings.Join(cols, ",") + ")"
}

// ============================================================
// Function 10: WriteDomainFiles
// ============================================================

// WriteDomainFiles writes the three domain atlas artifacts to disk.
func WriteDomainFiles(dbDir string, domainMap *DomainMap, enrichedMap map[string]*EnrichedTableEntry, crossDomainEdges []CrossDomainEdge) error {
	// Ensure schema directory exists
	schemaDir := filepath.Join(dbDir, "schema")
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		return fmt.Errorf("create schema dir: %w", err)
	}

	// Write domain_map.json to dbDir (top-level)
	domainMapData, err := json.MarshalIndent(domainMap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal domain_map: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dbDir, "domain_map.json"), domainMapData, 0644); err != nil {
		return fmt.Errorf("write domain_map.json: %w", err)
	}

	// Write schema/domain_map.json (enriched per-table)
	type enrichedOutput struct {
		Tables map[string]*EnrichedTableEntry `json:"tables"`
	}
	enrichedOutputData := enrichedOutput{Tables: enrichedMap}
	enrichedJSON, err := json.MarshalIndent(enrichedOutputData, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal enriched schema map: %w", err)
	}
	if err := os.WriteFile(filepath.Join(schemaDir, "domain_map.json"), enrichedJSON, 0644); err != nil {
		return fmt.Errorf("write schema/domain_map.json: %w", err)
	}

	// Update schema/join_graph.json with cross_domain annotations
	joinGraphPath := filepath.Join(schemaDir, "join_graph.json")
	joinGraphData, err := os.ReadFile(joinGraphPath)
	if err != nil {
		// If file doesn't exist yet, create it with just the cross-domain edges
		joinGraphJSON, err := json.MarshalIndent(map[string]any{"edges": crossDomainEdges}, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal join_graph: %w", err)
		}
		return os.WriteFile(joinGraphPath, joinGraphJSON, 0644)
	}

	// Existing file: parse, update cross_domain on each edge, write back
	type joinGraphFile struct {
		Edges     []map[string]any `json:"edges"`
		GeneratedAt string         `json:"generated_at"`
		TableCount  int            `json:"table_count"`
		DeclaredFKCount int        `json:"declared_fk_count,omitempty"`
		InferredJoinCount int      `json:"inferred_join_count,omitempty"`
	}

	var jgf joinGraphFile
	if err := json.Unmarshal(joinGraphData, &jgf); err != nil {
		return fmt.Errorf("parse existing join_graph.json: %w", err)
	}

	// Build lookup from the crossDomainEdges
	crossDomainLookup := make(map[string]bool)
	for _, ce := range crossDomainEdges {
		key := ce.From + "->" + ce.To
		crossDomainLookup[key] = ce.CrossDomain
	}

	// Update cross_domain on each edge
	for i, edge := range jgf.Edges {
		from, _ := edge["from"].(string)
		to, _ := edge["to"].(string)
		if from == "" {
			// Build from source/target
			source, _ := edge["source"].(string)
			target, _ := edge["target"].(string)
			if source != "" && target != "" {
				from = source
				to = target
			}
		}
		if from != "" && to != "" {
			key := from + "->" + to
			// Also try reverse
			revKey := to + "->" + from
			if cd, ok := crossDomainLookup[key]; ok {
				edge["cross_domain"] = cd
			} else if cd, ok := crossDomainLookup[revKey]; ok {
				edge["cross_domain"] = cd
			}
		}
		if _, ok := edge["cross_domain"]; !ok {
			edge["cross_domain"] = false
		}
		jgf.Edges[i] = edge
	}

	updatedData, err := json.MarshalIndent(jgf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal updated join_graph: %w", err)
	}
	return os.WriteFile(joinGraphPath, updatedData, 0644)
}

// ============================================================
// Function 11: LoadOverrides
// ============================================================

// LoadOverrides reads manual domain overrides from domain_overrides.json.
func LoadOverrides(dbDir string) map[string]string {
	overridesPath := filepath.Join(dbDir, "domain_overrides.json")
	data, err := os.ReadFile(overridesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string)
		}
		return make(map[string]string) // silent fallback on any read error
	}

	var of OverridesFile
	if err := json.Unmarshal(data, &of); err != nil {
		return make(map[string]string) // silent fallback on parse error
	}

	result := make(map[string]string, len(of.Overrides))
	for _, o := range of.Overrides {
		result[o.Table] = o.Domain
	}
	return result
}

// ============================================================
// ApplyOverrides applies manual overrides to the community map.
// ============================================================

// ApplyOverrides applies manual domain overrides to a communities map.
// Silent on overrides for non-existent tables.
func ApplyOverrides(communities map[string]int, communityNames map[int]string, overrides map[string]string) (map[string]int, map[int]string) {
	if len(overrides) == 0 {
		return communities, communityNames
	}

	// Find the next available community ID
	maxCID := 0
	for _, cid := range communities {
		if cid > maxCID {
			maxCID = cid
		}
	}

	// Build domain name -> community ID mapping
	nameToCID := make(map[string]int)
	for cid, name := range communityNames {
		nameToCID[name] = cid
	}

	newCommunities := make(map[string]int)
	for table, cid := range communities {
		newCommunities[table] = cid
	}

	for table, overrideDomain := range overrides {
		// Skip non-existent tables (silently ignored)
		if _, exists := communities[table]; !exists {
			continue
		}

		// Find or create community ID for the override domain
		if existingCID, ok := nameToCID[overrideDomain]; ok {
			newCommunities[table] = existingCID
		} else {
			maxCID++
			newCommunities[table] = maxCID
			nameToCID[overrideDomain] = maxCID
			communityNames[maxCID] = overrideDomain
		}
	}

	return newCommunities, communityNames
}

// ============================================================
// WriteSlimSchema writes the slim schema JSON to disk.
// ============================================================

// WriteSlimSchema marshals a SlimSchema to JSON and writes to
// <dbDir>/schema/schema_slim.json.
func WriteSlimSchema(dbDir string, slim *SlimSchema) error {
	// Marshal with sort keys for determinism
	data, err := json.MarshalIndent(slim, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal slim schema: %w", err)
	}

	schemaDir := filepath.Join(dbDir, "schema")
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		return fmt.Errorf("create schema dir: %w", err)
	}

	return os.WriteFile(filepath.Join(schemaDir, "schema_slim.json"), data, 0644)
}

// ============================================================
// Modularity classification
// ============================================================

// ClassifyModularity returns a string classification for a modularity score.
func ClassifyModularity(Q float64) string {
	if Q < 0.3 {
		return "weak"
	} else if Q < 0.5 {
		return "moderate"
	}
	return "strong"
}

// ============================================================
// ComputeSHA256 is a utility for manifest checksum computation
// ============================================================

// ComputeSHA256 computes the SHA-256 hex digest of data.
func ComputeSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// ============================================================
// SortMapKeys helper for deterministic JSON output
// ============================================================

// sortedKeys returns sorted string keys from a map.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
