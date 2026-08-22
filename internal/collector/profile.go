// Package collector orchestre la collective des profils bruts de
// l'application PHP cible (XHProf par défaut, php-spx / phpspy prévus).
//
// Ce fichier définit uniquement le contrat de données partagé avec
// internal/analyzer ; l'exécution réelle du profilage viendra plus tard.
package collector

import (
	"encoding/json"
	"fmt"
)

// RawProfile — profil brut au format XHProf : métriques indexées par
// relation d'appel.
//
// Chaque clé est :
//   - "main()"          → la trace racine ;
//   - "parent==>enfant" → coût de l'enfant lorsqu'appelé par parent.
//
// C'est la représentation telle que produite par XHProf (ou perftools/
// php-profiler) puis sérialisée en JSON par le pont PHP.
type RawProfile map[string]Entry

// Entry — métriques brutes XHProf d'une entrée du profil.
type Entry struct {
	CT  int64 `json:"ct"`  // nombre d'appels
	WT  int64 `json:"wt"`  // wall time, µs
	CPU int64 `json:"cpu"` // temps CPU, µs
	MU  int64 `json:"mu"`  // mémoire utilisée, octets
	PMU int64 `json:"pmu"` // pic mémoire, octets
}

// DecodeRaw — décode un profil brut depuis son JSON XHProf canonique.
func DecodeRaw(data []byte) (RawProfile, error) {
	var raw RawProfile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("collector: profil XHProf illisible : %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("collector: profil XHProf vide")
	}
	return raw, nil
}
