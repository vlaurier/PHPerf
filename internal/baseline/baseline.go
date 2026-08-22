// Package baseline persiste les findings connus d'un projet dans un fichier
// versionné (.phperf-baseline.json) et identifie les nouveaux : en CI,
// phperf-ci n'échoue que sur ces derniers (approche façon PHPStan).
//
// Le format v1 est minimal et déterministe — entrées triées par clé, aucun
// horodatage — pour que chaque régénération produise un diff git stable.
package baseline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/phperf/phperf/internal/rules"
)

// Version — format courant du fichier de baseline.
const Version = 1

// Entry — un finding connu de la baseline.
type Entry struct {
	Key      string `json:"key"`      // "<RuleID>|<callee>" — cf. rules.Finding
	RuleID   string `json:"rule_id"`  // redondant avec Key, pour la lisibilité du diff
	Function string `json:"function"` // callee concerné
}

// Baseline — contenu décodé du fichier .phperf-baseline.json.
type Baseline struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"` // triées par Key
}

// Keys — ensemble des clés connues de la baseline.
func (b *Baseline) Keys() map[string]struct{} {
	keys := make(map[string]struct{}, len(b.Entries))
	for _, e := range b.Entries {
		keys[e.Key] = struct{}{}
	}
	return keys
}

// Load — décode une baseline. Décodage strict : champ inconnu rejeté, seule
// la version courante est acceptée.
func Load(data []byte) (*Baseline, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var b Baseline
	if err := dec.Decode(&b); err != nil {
		return nil, fmt.Errorf("baseline : %w", err)
	}
	if b.Version != Version {
		return nil, fmt.Errorf("baseline : version %d non supportée (attendu : %d)", b.Version, Version)
	}
	return &b, nil
}

// Save — sérialise la baseline dans w : entrées triées par clé, JSON
// indenté, saut de ligne final. Le résultat ne dépend pas de l'ordre des
// entrées fournies ; une liste nil est normalisée en liste vide.
func Save(w io.Writer, b *Baseline) error {
	if b.Version != Version {
		return fmt.Errorf("baseline : version %d non supportée (attendu : %d)", b.Version, Version)
	}

	entries := make([]Entry, 0, len(b.Entries))
	entries = append(entries, b.Entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(&Baseline{Version: b.Version, Entries: entries}); err != nil {
		return fmt.Errorf("baseline : %w", err)
	}
	return nil
}

// Result — issue d'une comparaison findings ↔ baseline.
type Result struct {
	New   []rules.Finding // findings absents de la baseline, ordre conservé
	Known int             // findings déjà présents dans la baseline
}

// Diff — classe les findings courants selon la présence de leur clé dans
// la baseline.
func Diff(findings []rules.Finding, b *Baseline) Result {
	known := b.Keys()
	res := Result{New: make([]rules.Finding, 0, len(findings))}
	for _, f := range findings {
		if _, ok := known[f.Key]; ok {
			res.Known++
			continue
		}
		res.New = append(res.New, f)
	}
	return res
}
