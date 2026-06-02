package index

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/querylex/querylex/internal/db"
)

// Phase names for the indexing pipeline.
const (
	PhaseSchemaExtraction = "schema_extraction"
	PhaseJoinGraph        = "join_graph"
	PhaseSchemaMap        = "schema_map"
	PhaseDomainClustering     = "domain_clustering"
	PhaseTerminologyGeneration = "terminology_generation"
	PhaseOutputAssembly       = "output_assembly"
)

// Pipeline orchestrates the indexing phases for a database.
type Pipeline struct {
	adapter db.Adapter
	dbDir   string
	dbName  string
	dbType  string
}

// NewPipeline creates a new indexing pipeline.
func NewPipeline(adapter db.Adapter, dbDir string, dbName string, dbType string) *Pipeline {
	return &Pipeline{
		adapter: adapter,
		dbDir:   dbDir,
		dbName:  dbName,
		dbType:  dbType,
	}
}

// RunPipeline executes all indexing phases synchronously.
// It updates index_status.json at each phase transition.
func RunPipeline(ctx context.Context, p *Pipeline) error {
	// Initialize status
	status := NewIndexStatus("indexing", PhaseSchemaExtraction)
	if err := WriteIndexStatus(p.dbDir, status); err != nil {
		return fmt.Errorf("write initial status: %w", err)
	}

	// Phase 1: Schema Extraction (progress 0→15)
	result, err := p.extractSchema(ctx)
	if err != nil {
		p.failWithError(status, err)
		return err
	}
	status.CurrentPhase = PhaseJoinGraph
	status.ProgressPercent = 15
	if err := WriteIndexStatus(p.dbDir, status); err != nil {
		return fmt.Errorf("write status after schema extraction: %w", err)
	}

	// Phase 2: Join Graph (progress 15→30)
	joinGraph, err := p.buildJoinGraph(ctx, result)
	if err != nil {
		p.failWithError(status, err)
		return err
	}
	status.CurrentPhase = PhaseSchemaMap
	status.ProgressPercent = 30
	if err := WriteIndexStatus(p.dbDir, status); err != nil {
		return fmt.Errorf("write status after join graph: %w", err)
	}

	// Phase 3: Schema Map (progress 30→45)
	schemaMap, err := p.buildSchemaMap(ctx, result)
	if err != nil {
		p.failWithError(status, err)
		return err
	}
	status.CurrentPhase = PhaseDomainClustering
	status.ProgressPercent = 45
	if err := WriteIndexStatus(p.dbDir, status); err != nil {
		return fmt.Errorf("write status after schema map: %w", err)
	}

	// Phase 4: Domain Clustering (progress 45→75)
	if err := p.runDomainClustering(ctx, result, joinGraph, schemaMap); err != nil {
		p.failWithError(status, err)
		return err
	}

	// Phase 4.5: Terminology Generation (progress 75→85)
	status.CurrentPhase = PhaseTerminologyGeneration
	status.ProgressPercent = 75
	status.HeartbeatAt = time.Now().UTC().Format(time.RFC3339)
	if err := WriteIndexStatus(p.dbDir, status); err != nil {
		return fmt.Errorf("write status before terminology generation: %w", err)
	}
	// Non-fatal: terminology generation failure should not abort the pipeline
	if err := GenerateTerminologyTemplate(p.dbDir, result.Tables); err != nil {
		_ = err // best-effort — user can manually create the file
	}
	status.ProgressPercent = 85
	if err := WriteIndexStatus(p.dbDir, status); err != nil {
		return fmt.Errorf("write status after terminology generation: %w", err)
	}

	// Phase 5: Output Assembly (progress 85→100)
	status.CurrentPhase = PhaseOutputAssembly
	status.ProgressPercent = 85
	if err := WriteIndexStatus(p.dbDir, status); err != nil {
		return fmt.Errorf("write status before output assembly: %w", err)
	}
	if err := p.buildManifest(ctx, result); err != nil {
		p.failWithError(status, err)
		return err
	}
	status.Status = "indexed"
	status.CurrentPhase = PhaseOutputAssembly
	status.ProgressPercent = 100
	status.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	if err := WriteIndexStatus(p.dbDir, status); err != nil {
		return fmt.Errorf("write final status: %w", err)
	}

	return nil
}

// failWithError writes the failed status and returns the error.
func (p *Pipeline) failWithError(status *IndexStatus, err error) {
	status.Status = "index_failed"
	status.Error = err.Error()
	// Best-effort write — don't mask the original error
	_ = WriteIndexStatus(p.dbDir, status)
}

// extractSchema extracts and persists the database schema.
func (p *Pipeline) extractSchema(ctx context.Context) (*db.SchemaResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("schema extraction cancelled: %w", err)
	}

	result, err := p.adapter.Schema(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("schema extraction failed: %w", err)
	}

	// Write full schema JSON
	schemaData, err := BuildSchema(result)
	if err != nil {
		return nil, fmt.Errorf("build schema failed: %w", err)
	}
	if err := WriteSchema(p.dbDir, schemaData); err != nil {
		return nil, fmt.Errorf("write schema.json: %w", err)
	}

	// Write slim schema
	slimData, err := BuildSlimSchema(result)
	if err != nil {
		return nil, fmt.Errorf("build slim schema failed: %w", err)
	}
	schemaDir := filepath.Join(p.dbDir, "schema")
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		return nil, fmt.Errorf("create schema dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(schemaDir, "schema_slim.json"), slimData, 0644); err != nil {
		return nil, fmt.Errorf("write schema_slim.json: %w", err)
	}

	return result, nil
}

// buildJoinGraph builds and persists the join graph.
func (p *Pipeline) buildJoinGraph(ctx context.Context, result *db.SchemaResult) (*JoinGraphResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("join graph cancelled: %w", err)
	}

	graph, err := BuildJoinGraph(result)
	if err != nil {
		return nil, fmt.Errorf("build join graph failed: %w", err)
	}

	if err := WriteJoinGraph(p.dbDir, graph); err != nil {
		return nil, fmt.Errorf("write join_graph.json: %w", err)
	}

	return graph, nil
}

// buildSchemaMap builds and persists the schema map.
func (p *Pipeline) buildSchemaMap(ctx context.Context, result *db.SchemaResult) (SchemaMap, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("schema map cancelled: %w", err)
	}

	sm, err := BuildSchemaMap(result)
	if err != nil {
		return nil, fmt.Errorf("build schema map failed: %w", err)
	}

	if err := WriteSchemaMap(p.dbDir, sm); err != nil {
		return nil, fmt.Errorf("write schema_map.json: %w", err)
	}

	return sm, nil
}

// runDomainClustering executes the full domain atlas pipeline.
func (p *Pipeline) runDomainClustering(ctx context.Context, result *db.SchemaResult, joinGraph *JoinGraphResult, schemaMap SchemaMap) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("domain clustering cancelled: %w", err)
	}

	// Slim schema
	slim := TransformToSlimSchema(result, joinGraph)

	// Weighted graph
	graph, sortedNodes := BuildWeightedGraph(slim)

	// Louvain clustering
	communities := RunLouvain(graph, sortedNodes, 1.0)

	// Modularity
	modularity := ComputeModularity(graph, communities, 1.0)
	modularityClass := ClassifyModularity(modularity)

	// Name communities
	sortedTables := make([]string, 0, len(slim.Tables))
	for _, t := range slim.Tables {
		sortedTables = append(sortedTables, t.Name)
	}
	sort.Strings(sortedTables)

	communityNames := NameCommunities(communities, graph, sortedNodes, sortedTables)

	// Apply overrides
	overrides := LoadOverrides(p.dbDir)
	communities, communityNames = ApplyOverrides(communities, communityNames, overrides)

	// Sub-domains
	subDomainMap, domainSubDomains := DetectSubDomains(communities, communityNames, graph, sortedNodes, slim.Tables)

	// Bridge tables
	bridgeMap := DetectBridgeTables(slim, communities)

	// Domain map output
	domainMap := BuildDomainMapOutput(slim, communities, communityNames, subDomainMap, domainSubDomains, modularity, modularityClass)

	// Enriched schema map
	enrichedMap := BuildEnrichedSchemaMap(schemaMap, communities, communityNames, subDomainMap, bridgeMap)

	// Cross-domain annotations
	crossDomainEdges := AnnotateJoinGraphCrossDomain(joinGraph, communities)

	// Write domain files
	if err := WriteDomainFiles(p.dbDir, domainMap, enrichedMap, crossDomainEdges); err != nil {
		return fmt.Errorf("write domain files failed: %w", err)
	}

	return nil
}

// buildManifest writes the index manifest.
func (p *Pipeline) buildManifest(ctx context.Context, result *db.SchemaResult) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("output assembly cancelled: %w", err)
	}

	tableCount := 0
	if result != nil {
		tableCount = len(result.Tables)
	}

	manifest, err := BuildManifest(p.dbDir, p.dbType, tableCount,
		"schema/schema.json",
		"schema/schema_slim.json",
		"schema/join_graph.json",
		"domain_map.json",
		"schema/domain_map.json",
	)
	if err != nil {
		return fmt.Errorf("build manifest failed: %w", err)
	}

	if err := WriteIndexManifest(p.dbDir, manifest); err != nil {
		return fmt.Errorf("write index manifest: %w", err)
	}

	return nil
}
