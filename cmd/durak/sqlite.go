package main

import (
	"context"

	"github.com/vovakirdan/durak/internal/adapters/storage"
)

func withSQLiteStore(ctx context.Context, path string, run func(*storage.SQLiteStore) error) error {
	store, err := storage.OpenSQLiteStore(ctx, path)
	if err != nil {
		return err
	}
	return closeSQLiteStore(store, run(store))
}

func closeSQLiteStore(store *storage.SQLiteStore, runErr error) error {
	if store == nil {
		return runErr
	}
	if closeErr := store.Close(); runErr == nil && closeErr != nil {
		return closeErr
	}
	return runErr
}
