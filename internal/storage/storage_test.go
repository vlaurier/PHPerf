package storage_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phperf/phperf/internal/storage"
)

// openDB — ouvre une base temporaire fermée automatiquement en fin de test.
func openDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestOpen_Memory(t *testing.T) {
	db, err := storage.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, db.AddMask("a|x"))
	keys, err := db.MaskedKeys()
	require.NoError(t, err)
	assert.Equal(t, map[string]struct{}{"a|x": {}}, keys)
}

func TestAddRemoveMask_RoundTrip(t *testing.T) {
	db := openDB(t)

	require.NoError(t, db.AddMask("n-plus-one-query|PDO::query"))
	require.NoError(t, db.AddMask("duplicated-calculation|PDO::query"))

	keys, err := db.MaskedKeys()
	require.NoError(t, err)
	assert.Equal(t, map[string]struct{}{
		"n-plus-one-query|PDO::query":       {},
		"duplicated-calculation|PDO::query": {},
	}, keys)

	// Démasquage sélectif.
	require.NoError(t, db.RemoveMask("duplicated-calculation|PDO::query"))
	keys, err = db.MaskedKeys()
	require.NoError(t, err)
	assert.Equal(t, map[string]struct{}{"n-plus-one-query|PDO::query": {}}, keys)

	// Démasquer une clé absente : no-op.
	require.NoError(t, db.RemoveMask("jamais-masquée"))
}

func TestMasks_PersistAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persist.db")

	db, err := storage.Open(path)
	require.NoError(t, err)
	require.NoError(t, db.AddMask("r|f"))
	require.NoError(t, db.Close())

	reopened, err := storage.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reopened.Close() })

	keys, err := reopened.MaskedKeys()
	require.NoError(t, err)
	assert.Equal(t, map[string]struct{}{"r|f": {}}, keys)
}

func TestOpen_UnwritablePath(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "nope", "sub", "db.sqlite"))
	require.Error(t, err)
	assert.Nil(t, db)
}
