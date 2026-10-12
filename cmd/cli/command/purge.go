package command

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

const (
	usersTable     = "users"
	migrationTable = "goose_db_version"
)

var tableNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func NewPurgeCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "purge",
		Short:        "Empty every table except users and goose_db_version",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			sqlDB, err := openSQL()
			if err != nil {
				return err
			}
			defer sqlDB.Close()
			return runPurge(cmd.Context(), sqlDB, cmd.OutOrStdout())
		},
	}
}

func runPurge(ctx context.Context, db *sql.DB, out io.Writer) error {
	purged, err := purgeDatabase(ctx, db)
	if err != nil {
		return err
	}
	if len(purged) == 0 {
		fmt.Fprintln(out, "nothing to purge")
		return nil
	}
	fmt.Fprintf(out, "purged: %s\n", strings.Join(purged, ", "))
	fmt.Fprintf(out, "kept: %s, %s\n", usersTable, migrationTable)
	return nil
}

func purgeDatabase(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		ORDER BY tablename
	`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("list tables: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}

	targets := tablesToPurge(names)
	if len(targets) == 0 {
		return nil, nil
	}
	quoted := make([]string, len(targets))
	for i, name := range targets {
		quoted[i], err = quoteTable(name)
		if err != nil {
			return nil, err
		}
	}
	query := "TRUNCATE TABLE " + strings.Join(quoted, ", ") + " RESTART IDENTITY"
	if _, err := db.ExecContext(ctx, query); err != nil {
		return nil, fmt.Errorf("purge tables: %w", err)
	}
	return targets, nil
}

func tablesToPurge(names []string) []string {
	kept := map[string]struct{}{
		usersTable:     {},
		migrationTable: {},
	}
	targets := make([]string, 0, len(names))
	for _, name := range names {
		if _, skip := kept[name]; skip {
			continue
		}
		targets = append(targets, name)
	}
	return targets
}

func quoteTable(name string) (string, error) {
	if !tableNamePattern.MatchString(name) {
		return "", fmt.Errorf("refuse to purge table %q", name)
	}
	return `"` + name + `"`, nil
}
