package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/querylex/querylex/internal/format"
	"github.com/querylex/querylex/internal/memory"
)

// SaveData is the response data for the save command.
type SaveData struct {
	Saved          bool           `json:"saved"`
	UpdatedExisting bool          `json:"updated_existing"`
	Entry          *SaveEntryData `json:"entry"`
}

// SaveEntryData contains the entry details returned by the save command.
type SaveEntryData struct {
	ID         string `json:"id"`
	Input      string `json:"input"`
	SQLHash    string `json:"sql_hash"`
	DatabaseID string `json:"database_id"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// RunSave executes the querylex save command.
// It upserts a memory entry, rebuilds the index, and returns the result.
func RunSave(input, sql string) *format.Response[SaveData] {
	start := time.Now()
	traceID := format.GenerateTraceID()

	preflight, errResp := PreflightForMemoryCommand()
	if errResp != nil {
		resp := convertSaveError(errResp)
		resp.Complete(start)
		return resp
	}

	// Open memory store
	db, err := memory.OpenStore(preflight.DBDir)
	if err != nil {
		resp := format.NewErrorResponse[SaveData](
			format.ErrCodeMemoryStoreUnavailable,
			fmt.Sprintf("Memory subsystem is unavailable: %v", err),
			true,
			traceID,
		)
		if resp.Error != nil {
			resp.Error.Details = map[string]string{
				"store": filepath.Join(preflight.DBDir, "memory.sqlite"),
			}
		}
		resp.Complete(start)
		return resp
	}
	defer db.Close()

	// Save the entry
	ctx := context.Background()
	entry, updatedExisting, err := memory.SaveEntry(ctx, db, input, sql, preflight.ActiveDBID)
	if err != nil {
		resp := format.NewErrorResponse[SaveData](
			format.ErrCodeMemoryWriteFailed,
			fmt.Sprintf("Unable to write memory entry: %v", err),
			true,
			traceID,
		)
		if resp.Error != nil {
			resp.Error.Details = map[string]string{
				"store": filepath.Join(preflight.DBDir, "memory.sqlite"),
			}
		}
		resp.Complete(start)
		return resp
	}

	// Rebuild index after successful save
	warning := addEmbeddingsWarning()
	if allEntries, listErr := memory.ListEntries(ctx, db); listErr == nil {
		if rebuiltIndex, rebuildErr := memory.RebuildIndex(preflight.DBDir, allEntries); rebuildErr == nil {
			if revision, revErr := memory.GetRevision(ctx, db); revErr == nil {
				rebuiltIndex.Revision = revision
				if writeErr := memory.WriteIndex(preflight.DBDir, rebuiltIndex); writeErr != nil {
					// Index write failed — add warning but save is still durable
					warning = append(warning, format.Warning{
						Code:    "MEMORY_INDEX_STALE",
						Message: "Memory index metadata is stale relative to memory.sqlite.",
					})
				}
			}
		}
	}

	data := SaveData{
		Saved:           true,
		UpdatedExisting: updatedExisting,
		Entry: &SaveEntryData{
			ID:         entry.ID,
			Input:      entry.Input,
			SQLHash:    entry.SQLHash,
			DatabaseID: entry.DatabaseID,
			CreatedAt:  entry.CreatedAt,
			UpdatedAt:  entry.UpdatedAt,
		},
	}

	resp := format.NewSuccessResponse(data, traceID, &preflight.ActiveDBID)
	resp.Warnings = append(resp.Warnings, warning...)
	resp.Complete(start)
	return resp
}

// addEmbeddingsWarning returns the standard EMBEDDINGS_UNAVAILABLE warning.
func addEmbeddingsWarning() []format.Warning {
	return []format.Warning{
		{
			Code:    "EMBEDDINGS_UNAVAILABLE",
			Message: "Similarity scoring uses lexical methods; embeddings unavailable. Results may have lower precision.",
		},
	}
}

// convertSaveError converts an any-typed error response to a SaveData-typed one.
func convertSaveError(errResp *format.Response[any]) *format.Response[SaveData] {
	if errResp.Error != nil {
		return format.NewErrorResponse[SaveData](
			errResp.Error.Code,
			errResp.Error.Message,
			errResp.Error.Retryable,
			errResp.Meta.TraceID,
		)
	}
	return format.NewErrorResponse[SaveData](
		format.ErrCodeInternalError,
		"Unknown error during preflight",
		false,
		format.GenerateTraceID(),
	)
}
