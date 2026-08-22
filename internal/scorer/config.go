package scorer

import (
	"bytes"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	"github.com/phperf/phperf/internal/rules"
)

// LoadWeights — décode une configuration de pondérations YAML et la fusionne
// avec les défauts : toute clé absente du fichier garde sa valeur par défaut.
//
// Décodage strict (champ inconnu rejeté, comme pour les règles) ; chaque
// poids présent doit être strictement positif. Un fichier vide est une erreur
// : l'absence de configuration se gère côté appelant via DefaultWeights.
func LoadWeights(data []byte) (WeightSet, error) {
	w := DefaultWeights()

	var file struct {
		Scoring struct {
			Weights struct {
				Effort          map[rules.Effort]float64          `yaml:"effort"`
				Controllability map[rules.Controllability]float64 `yaml:"controllability"`
			} `yaml:"weights"`
		} `yaml:"scoring"`
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&file); err != nil {
		if err == io.EOF {
			return WeightSet{}, fmt.Errorf("scorer : fichier de scoring vide")
		}
		return WeightSet{}, fmt.Errorf("scorer : %w", err)
	}

	if err := mergeWeights(w.Effort, file.Scoring.Weights.Effort, "effort"); err != nil {
		return WeightSet{}, err
	}
	if err := mergeWeights(w.Controllability, file.Scoring.Weights.Controllability, "controllability"); err != nil {
		return WeightSet{}, err
	}
	return w, nil
}

// mergeWeights — fusionne les surcharges src dans dst après validation :
// clé inconnue ou valeur ≤ 0 rejetées. kind sert au message d'erreur.
func mergeWeights[K comparable](dst, src map[K]float64, kind string) error {
	for k, v := range src {
		if _, ok := dst[k]; !ok {
			return fmt.Errorf("scorer : %s %v inconnu", kind, k)
		}
		if v <= 0 {
			return fmt.Errorf("scorer : poids %s %v invalide (%g) : strictement positif requis", kind, k, v)
		}
		dst[k] = v
	}
	return nil
}
