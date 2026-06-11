package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cskiller24/querylex/internal/credentials"
	"github.com/cskiller24/querylex/internal/db"
	_ "github.com/cskiller24/querylex/internal/db/mysql"
	"github.com/cskiller24/querylex/internal/index"
	"github.com/cskiller24/querylex/internal/state"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	dbID := "tpch-mysql"
	dbName := "TPC-H MySQL"
	dbType := "mysql"
	host := "localhost"
	port := 3307
	database := "tpch"
	username := "tpch"
	password := "tpch"
	sslMode := "disable"

	querylexDir := filepath.Join(home, ".querylex")
	dbDir := filepath.Join(querylexDir, dbID)

	// Store credential
	credStore, err := credentials.SelectCredentialStore()
	if err != nil {
		panic(fmt.Sprintf("credential store: %v", err))
	}

	credRef, err := credStore.Store(dbID, password)
	if err != nil {
		panic(fmt.Sprintf("store credential: %v", err))
	}
	credRef.SecretKind = "database-password"
	fmt.Printf("Credential stored: %+v\n", credRef)

	// Build DSN and verify connection
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?tls=%s&timeout=10s", username, password, host, port, database, sslMode)
	if sslMode == "disable" {
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?tls=false&timeout=10s", username, password, host, port, database)
	}

	adapter, err := db.Open(dbType, dsn)
	if err != nil {
		panic(fmt.Sprintf("open adapter: %v", err))
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := adapter.Connect(pingCtx, dsn); err != nil {
		panic(fmt.Sprintf("connect: %v", err))
	}
	fmt.Println("Connection successful!")
	adapter.Close(context.Background())

	// Create database directory
	if err := os.MkdirAll(dbDir, 0700); err != nil {
		panic(fmt.Sprintf("mkdir: %v", err))
	}

	// Write database.json
	type dbConnectionMeta struct {
		ID            string                        `json:"id"`
		Name          string                        `json:"name"`
		Type          string                        `json:"type"`
		Host          string                        `json:"host"`
		Port          int                           `json:"port"`
		Database      string                        `json:"database"`
		Username      string                        `json:"username"`
		SSLMode       string                        `json:"ssl_mode"`
		CredentialRef *credentials.CredentialReference `json:"credential_reference"`
	}

	meta := dbConnectionMeta{
		ID:            dbID,
		Name:          dbName,
		Type:          dbType,
		Host:          host,
		Port:          port,
		Database:      database,
		Username:      username,
		SSLMode:       sslMode,
		CredentialRef: credRef,
	}

	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("marshal: %v", err))
	}

	if err := os.WriteFile(filepath.Join(dbDir, "database.json"), metaData, 0600); err != nil {
		panic(fmt.Sprintf("write database.json: %v", err))
	}
	fmt.Println("database.json written")

	// Update workspace
	workspaceFile := filepath.Join(querylexDir, "querylex.json")
	wsStore := state.NewFileWorkspaceStore(workspaceFile)
	entry := state.DatabaseEntry{
		ID:               dbID,
		Name:             dbName,
		Type:             dbType,
		Status:           state.StatusNotIndexed,
		IndexingProgress: 0,
	}

	if err := wsStore.AddDatabase(entry); err != nil {
		// Check if already exists
		if strings.Contains(err.Error(), "already exists") {
			fmt.Println("Database already exists in workspace")
		} else {
			panic(fmt.Sprintf("add database: %v", err))
		}
	}

	if err := wsStore.SetActiveDatabase(dbID); err != nil {
		panic(fmt.Sprintf("set active: %v", err))
	}
	fmt.Println("Workspace updated")

	// Run indexing pipeline
	fmt.Println("Starting indexing...")
	adapter2, err := db.Open(dbType, dsn)
	if err != nil {
		panic(fmt.Sprintf("open adapter for indexing: %v", err))
	}
	if err := adapter2.Connect(pingCtx, dsn); err != nil {
		panic(fmt.Sprintf("connect for indexing: %v", err))
	}

	pipeline := index.NewPipeline(adapter2, dbDir, dbName, dbType)
	pipeCtx, pipeCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	if pipeErr := index.RunPipeline(pipeCtx, pipeline); pipeErr != nil {
		fmt.Printf("Indexing failed: %v\n", pipeErr)
		indexingStatus := "index_failed"
		indexingProgress := 0
		if st, readErr := index.ReadIndexStatus(dbDir); readErr == nil && st != nil {
			indexingProgress = st.ProgressPercent
		}
		if ws, loadErr := wsStore.Load(); loadErr == nil {
			for i := range ws.ConnectedDatabases {
				if ws.ConnectedDatabases[i].ID == dbID {
					ws.ConnectedDatabases[i].Status = state.DatabaseStatus(indexingStatus)
					ws.ConnectedDatabases[i].IndexingProgress = indexingProgress
					_ = wsStore.Save(ws)
					break
				}
			}
		}
	} else {
		fmt.Println("Indexing complete!")
		indexingStatus := "indexed"
		if ws, loadErr := wsStore.Load(); loadErr == nil {
			for i := range ws.ConnectedDatabases {
				if ws.ConnectedDatabases[i].ID == dbID {
					ws.ConnectedDatabases[i].Status = state.DatabaseStatus(indexingStatus)
					ws.ConnectedDatabases[i].IndexingProgress = 100
					_ = wsStore.Save(ws)
					break
				}
			}
		}
	}
	pipeCancel()
	adapter2.Close(context.Background())

	fmt.Println("Done!")
}
