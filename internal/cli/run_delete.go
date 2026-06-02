package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/querylex/querylex/internal/format"
	"github.com/querylex/querylex/internal/memory"
)

// DeleteData is the response data for the delete command.
type DeleteData struct {
	Deleted bool            `json:"deleted"`
	Entry   *DeleteEntryData `json:"entry"`
}

// DeleteEntryData contains the entry details returned by the delete command.
type DeleteEntryData struct {
	ID         string `json:"id"`
	Input      string `json:"input"`
	DatabaseID string `json:"database_id"`
}

// RunDelete executes the querylex delete command.
// It removes a memory entry by normalized input and rebuilds the index.
// Deleting a non-existent entry is a successful no-op (CMND-13).
func RunDelete(input string) *format.Response[DeleteData] {
	start := time.Now()
	traceID := format.GenerateTraceID()

	preflight, errResp := PreflightForMemoryCommand()
	if errResp != nil {
		resp := convertDeleteError(errResp)
		resp.Complete(start)
		return resp
	}

	// Open memory store
	db, err := memory.OpenStore(preflight.DBDir)
	if err != nil {
		resp := format.NewErrorResponse[DeleteData](
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

	ctx := context.Background()
	normalizedInput := memory.NormalizeInput(input)

	// Check if entry exists
	entry, err := memory.GetEntry(ctx, db, normalizedInput)
	if err != nil {
		resp := format.NewErrorResponse[DeleteData](
			format.ErrCodeMemoryStoreUnavailable,
			fmt.Sprintf("Failed to look up entry: %v", err),
			true,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	// Entry not found — successful no-op per CMND-13
	if entry == nil {
		data := DeleteData{
			Deleted: false,
			Entry:   nil,
		}
		resp := format.NewSuccessResponse(data, traceID, &preflight.ActiveDBID)
		resp.Complete(start)
		return resp
	}

	// Capture entry details before deletion
	entryDetails := &DeleteEntryData{
		ID:         entry.ID,
		Input:      entry.Input,
		DatabaseID: entry.DatabaseID,
	}

	// Delete the entry
	deleted, err := memory.DeleteEntry(ctx, db, normalizedInput)
	if err != nil {
		resp := format.NewErrorResponse[DeleteData](
			format.ErrCodeMemoryWriteFailed,
			fmt.Sprintf("Failed to delete entry: %v", err),
			true,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	if !deleted {
		// Should not happen since GetEntry found it — handle gracefully
		data := DeleteData{
			Deleted: false,
			Entry:   nil,
		}
		resp := format.NewSuccessResponse(data, traceID, &preflight.ActiveDBID)
		resp.Complete(start)
		return resp
	}

	// Rebuild index after successful delete
	var warnings []format.Warning
	if allEntries, listErr := memory.ListEntries(ctx, db); listErr == nil {
		if rebuiltIndex, rebuildErr := memory.RebuildIndex(preflight.DBDir, allEntries); rebuildErr == nil {
			if revision, revErr := memory.GetRevision(ctx, db); revErr == nil {
				rebuiltIndex.Revision = revision
				if writeErr := memory.WriteIndex(preflight.DBDir, rebuiltIndex); writeErr != nil {
					warnings = append(warnings, format.Warning{
						Code:    "MEMORY_INDEX_STALE",
						Message: "Memory index metadata is stale relative to memory.sqlite.",
					})
				}
			}
		}
	}

	data := DeleteData{
		Deleted: true,
		Entry:   entryDetails,
	}

	resp := format.NewSuccessResponse(data, traceID, &preflight.ActiveDBID)
	resp.Warnings = append(resp.Warnings, warnings...)
	resp.Complete(start)
	return resp
}

// convertDeleteError converts an any-typed error response to a DeleteData-typed one.
func convertDeleteError(errResp *format.Response[any]) *format.Response[DeleteData] {
	if errResp.Error != nil {
		return format.NewErrorResponse[DeleteData](
			errResp.Error.Code,
			errResp.Error.Message,
			errResp.Error.Retryable,
			errResp.Meta.TraceID,
		)
	}
	return format.NewErrorResponse[DeleteData](
		format.ErrCodeInternalError,
		"Unknown error during preflight",
		false,
		format.GenerateTraceID(),
	)
}
