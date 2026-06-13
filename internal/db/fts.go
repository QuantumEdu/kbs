package db

import (
	"context"
	"fmt"
)

func (s *sqliteSearchStore) RebuildFTS(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO entries_fts(entries_fts) VALUES('rebuild')")
	if err != nil {
		return fmt.Errorf("rebuild fts: %w", err)
	}
	return nil
}
