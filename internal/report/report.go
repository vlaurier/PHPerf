// Package report rend les résultats PHPerf consommables : page HTML pour
// l'UI web et export JSON machine-lisible.
//
// Le package définit ses propres vues (DTO) et n'importe aucun package
// métier : le wiring cmd/* convertit les scores en report.Finding. Ainsi
// présentation et calcul restent découplés.
package report

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Finding — vue aplatie d'un finding scoré, prête à afficher.
type Finding struct {
	Key             string // clé stable « <RuleID>|<callee> »
	RuleID          string
	Function        string
	Caller          string
	Severity        string // critical|high|medium|low (affichage seul)
	Effort          string // low|medium|high
	Controllability string // controllable|partial|none
	Recommendation  string
	CallCount       int64
	TimeShare       float64 // part du wall time du callee (0–1)
	Priority        float64 // 0–100
	Masked          bool    // décision de triage locale
}

// Data — tout ce qu'un rendu a besoin.
type Data struct {
	Title       string
	Findings    []Finding // triés par priorité décroissante côté appelant
	MaskedCount int       // nb total de masques (pour le lien d'affichage)
	ShowMasked  bool      // inclure les findings masqués dans la liste ?
}

// JSON — export machine-lisible, indenté, saut de ligne final.
func JSON(data Data) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(data); err != nil {
		return nil, fmt.Errorf("report : %w", err)
	}
	return buf.Bytes(), nil
}
