package index

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestIndexManifest_WriteReadRoundtrip(t *testing.T) {
	dir := t.TempDir()

	manifest := &IndexManifest{
		SchemaVersionHash: "abc123def456",
		DBVersion:         "mysql",
		TableCount:        15,
		ArtifactChecksums: map[string]string{
			"schema/schema.json":    "abc",
			"schema/join_graph.json": "def",
		},
		GeneratedAt: "2024-01-15T10:30:00Z",
	}

	if err := WriteIndexManifest(dir, manifest); err != nil {
		t.Fatalf("WriteIndexManifest failed: %v", err)
	}

	// Verify file exists
	manifestPath := filepath.Join(dir, "indexes", "index_manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatal("index_manifest.json not written")
	}

	// Read back
	readManifest, err := ReadIndexManifest(dir)
	if err != nil {
		t.Fatalf("ReadIndexManifest failed: %v", err)
	}
	if readManifest == nil {
		t.Fatal("ReadIndexManifest returned nil")
	}
	if readManifest.SchemaVersionHash != "abc123def456" {
		t.Errorf("expected SchemaVersionHash='abc123def456', got '%s'", readManifest.SchemaVersionHash)
	}
	if readManifest.DBVersion != "mysql" {
		t.Errorf("expected DBVersion='mysql', got '%s'", readManifest.DBVersion)
	}
	if readManifest.TableCount != 15 {
		t.Errorf("expected TableCount=15, got %d", readManifest.TableCount)
	}
	if len(readManifest.ArtifactChecksums) != 2 {
		t.Errorf("expected 2 artifact checksums, got %d", len(readManifest.ArtifactChecksums))
	}
}

func TestIndexManifest_MissingFile(t *testing.T) {
	dir := t.TempDir()

	manifest, err := ReadIndexManifest(dir)
	if err != nil {
		t.Fatalf("ReadIndexManifest on missing file failed: %v", err)
	}
	if manifest != nil {
		t.Fatal("expected nil for missing manifest file")
	}
}

func TestComputeChecksum(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	content := "hello world\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	checksum, err := ComputeChecksum(testFile)
	if err != nil {
		t.Fatalf("ComputeChecksum failed: %v", err)
	}

	// Verify using standard library
	hash := sha256.Sum256([]byte(content))
	expected := hex.EncodeToString(hash[:])
	if checksum != expected {
		t.Errorf("checksum mismatch: got '%s', expected '%s'", checksum, expected)
	}
}

func TestComputeChecksum_NonexistentFile(t *testing.T) {
	_, err := ComputeChecksum("/nonexistent/path/file.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestVerifyManifest_AllMatch(t *testing.T) {
	dir := t.TempDir()

	// Create actual artifact files
	schemaContent := []byte(`{"tables":[{"name":"users"}]}`)
	schemaPath := filepath.Join(dir, "schema", "schema.json")
	os.MkdirAll(filepath.Dir(schemaPath), 0755)
	if err := os.WriteFile(schemaPath, schemaContent, 0644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	schemaHash := sha256Hex(schemaContent)

	manifest := &IndexManifest{
		SchemaVersionHash: "testhash",
		DBVersion:         "mysql",
		TableCount:        1,
		ArtifactChecksums: map[string]string{
			"schema/schema.json": schemaHash,
		},
		GeneratedAt: "2024-01-15T10:30:00Z",
	}

	ok, err := VerifyManifest(dir, manifest)
	if err != nil {
		t.Fatalf("VerifyManifest failed: %v", err)
	}
	if !ok {
		t.Error("expected VerifyManifest=true when all checksums match")
	}
}

func TestVerifyManifest_Mismatch(t *testing.T) {
	dir := t.TempDir()

	schemaContent := []byte(`{"tables":[{"name":"users"}]}`)
	schemaPath := filepath.Join(dir, "schema", "schema.json")
	os.MkdirAll(filepath.Dir(schemaPath), 0755)
	if err := os.WriteFile(schemaPath, schemaContent, 0644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	manifest := &IndexManifest{
		ArtifactChecksums: map[string]string{
			"schema/schema.json": "wronghash",
		},
	}

	ok, err := VerifyManifest(dir, manifest)
	if err != nil {
		t.Fatalf("VerifyManifest failed: %v", err)
	}
	if ok {
		t.Error("expected VerifyManifest=false when checksum mismatches")
	}
}

func TestVerifyManifest_MissingArtifact(t *testing.T) {
	dir := t.TempDir()

	manifest := &IndexManifest{
		ArtifactChecksums: map[string]string{
			"schema/schema.json": "anyhash",
		},
	}

	ok, err := VerifyManifest(dir, manifest)
	if err != nil {
		t.Fatalf("VerifyManifest failed: %v", err)
	}
	if ok {
		t.Error("expected VerifyManifest=false when artifact file is missing")
	}
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
