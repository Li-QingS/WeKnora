package database

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

var migrationFilePattern = regexp.MustCompile(`^(\d{6})_(.+)\.(up|down)\.sql$`)

// TestMigrationVersionsAreUniqueAndPaired prevents a silent Git merge from
// leaving two migrations with the same numeric version. golang-migrate rejects
// that directory at runtime even when the SQL files themselves do not conflict.
func TestMigrationVersionsAreUniqueAndPaired(t *testing.T) {
	repoRoot := sqliteRepoRoot(t)
	for _, relativeDir := range []string{"migrations/versioned", "migrations/sqlite"} {
		t.Run(relativeDir, func(t *testing.T) {
			entries, err := os.ReadDir(filepath.Join(repoRoot, relativeDir))
			require.NoError(t, err)

			type migrationPair struct {
				name string
				up   string
				down string
			}
			versions := make(map[string]*migrationPair)
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				matches := migrationFilePattern.FindStringSubmatch(entry.Name())
				if matches == nil {
					continue
				}
				version, name, direction := matches[1], matches[2], matches[3]
				pair := versions[version]
				if pair == nil {
					pair = &migrationPair{name: name}
					versions[version] = pair
				}
				require.Equalf(t, pair.name, name, "migration version %s is used by multiple names", version)
				if direction == "up" {
					require.Emptyf(t, pair.up, "migration version %s has duplicate up files", version)
					pair.up = entry.Name()
				} else {
					require.Emptyf(t, pair.down, "migration version %s has duplicate down files", version)
					pair.down = entry.Name()
				}
			}

			for version, pair := range versions {
				require.NotEmptyf(t, pair.up, "migration version %s is missing its up file", version)
				require.NotEmptyf(t, pair.down, "migration version %s is missing its down file", version)
			}
		})
	}
}
