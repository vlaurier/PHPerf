// Package storage persiste les décisions de triage de PHPerf dans une base
// SQLite fichier unique (driver pur Go modernc.org/sqlite : pas de CGO,
// binaire portable).
//
// Le package définit ses propres modèles et n'importe aucun package métier :
// les clés de findings (« <RuleID>|<callee> ») y sont traitées comme des
// chaînes opaques.
package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // driver database/sql, sans CGO
)

// schema — migrations idempotentes, appliquées à chaque ouverture.
const schema = `
CREATE TABLE IF NOT EXISTS masks (
	key        TEXT PRIMARY KEY,
	created_at TEXT NOT NULL -- RFC 3339, UTC
);
`

// ErrNotFound — la clé demandée n'existe pas (réservé aux futurs usages ;
// AddMask et RemoveMask sont idempotents par conception).
var ErrNotFound = errors.New("storage : introuvable")

// DB — accès SQLite aux décisions de triage.
type DB struct {
	sql *sql.DB
}

// Open — ouvre (et crée au besoin) la base au chemin donné puis applique
// les migrations. Le chemin spécial « :memory: » crée une base éphémère.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("storage : %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("storage : migration : %w", err)
	}
	return &DB{sql: db}, nil
}

// Close — ferme la base.
func (d *DB) Close() error {
	return d.sql.Close()
}

// AddMask — masque une clé (finding jugé sans objet : faux positif,
// won't fix…). Masquer une clé déjà masquée est un no-op.
func (d *DB) AddMask(key string) error {
	const q = `INSERT INTO masks (key, created_at) VALUES (?, ?) ON CONFLICT(key) DO NOTHING`
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := d.sql.Exec(q, key, now); err != nil {
		return fmt.Errorf("storage : masquage %q : %w", key, err)
	}
	return nil
}

// RemoveMask — démasque une clé. Démasquer une clé absente est un no-op.
func (d *DB) RemoveMask(key string) error {
	if _, err := d.sql.Exec(`DELETE FROM masks WHERE key = ?`, key); err != nil {
		return fmt.Errorf("storage : démasquage %q : %w", key, err)
	}
	return nil
}

// MaskedKeys — ensemble des clés actuellement masquées, triées.
func (d *DB) MaskedKeys() (map[string]struct{}, error) {
	rows, err := d.sql.Query(`SELECT key FROM masks ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("storage : lecture des masques : %w", err)
	}
	defer func() { _ = rows.Close() }()

	keys := make(map[string]struct{})
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("storage : lecture des masques : %w", err)
		}
		keys[key] = struct{}{}
	}
	return keys, rows.Err()
}
