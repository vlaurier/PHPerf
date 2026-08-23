// Tests boîte blanche : injection de pannes driver impossibles à provoquer
// depuis l'extérieur (le conteneur de dev tourne en root, les permissions
// de fichier sont ignorées). On ferme le pool sous-jacent : toute opération
// échoue et couvre les branches d'erreur.
package storage

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOps_DriverErrors(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	require.NoError(t, conn.Close()) // pool fermé → chaque requête échoue

	d := &DB{sql: conn}

	assert.Error(t, d.AddMask("k"))
	assert.Error(t, d.RemoveMask("k"))

	keys, err := d.MaskedKeys()
	assert.Nil(t, keys)
	assert.Error(t, err)
}
