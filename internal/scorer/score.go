// Package scorer attribue une priorité (0–100) aux findings produits par le
// moteur de règles : Impact × Effort × Contrôlabilité.
//
// Formule v1 :
//
//	Priority = 100 × Evidence.TimeShare × poids_effort × poids_contrôlabilité
//
// Le temps inclusif du callee agrège déjà ses répétitions (50 requêtes N+1
// pèsent lourd dans son wall time) : aucun multiplicateur supplémentaire par
// nombre d'appels n'est appliqué — ce serait compter deux fois le même coût.
// Décision et rationale : docs/jalons.md.
//
// Les pondérations sont ajustables via un fichier YAML (modèle :
// proto/scoring.example.yaml) ; toute clé absente garde sa valeur par défaut.
package scorer

import (
	"fmt"
	"sort"

	"github.com/phperf/phperf/internal/rules"
)

const maxPriority = 100.0

// WeightSet — pondérations du scoreur, indexées par effort et par niveau de
// contrôlabilité. Toutes les valeurs doivent être strictement positives.
type WeightSet struct {
	Effort          map[rules.Effort]float64
	Controllability map[rules.Controllability]float64
}

// Scored — finding accompagné de sa priorité calculée.
type Scored struct {
	Finding  rules.Finding
	Priority float64 // 0–100 ; ordre décroissant dans Score
}

// Scorer — calcule la priorité des findings d'une trace.
type Scorer interface {
	Score(findings []rules.Finding) []Scored
}

// DefaultWeights — pondérations embarquées, utilisées quand aucune
// configuration n'est fournie : effort faible ⇒ poids fort (corriger vite
// est prioritaire), maîtrise totale ⇒ poids fort.
func DefaultWeights() WeightSet {
	return WeightSet{
		Effort: map[rules.Effort]float64{
			rules.EffortLow:    1.0,
			rules.EffortMedium: 0.6,
			rules.EffortHigh:   0.3,
		},
		Controllability: map[rules.Controllability]float64{
			rules.Controllable:   1.0,
			rules.PartialControl: 0.6,
			rules.NoControl:      0.3,
		},
	}
}

// Engine — Scorer appliquant un jeu de pondérations fixé à la construction.
type Engine struct {
	weights WeightSet
}

// NewEngine — valide le jeu de pondérations puis construit le scoreur.
func NewEngine(weights WeightSet) (*Engine, error) {
	for _, e := range []rules.Effort{rules.EffortLow, rules.EffortMedium, rules.EffortHigh} {
		if err := positiveWeight("effort", string(e), weights.Effort[e]); err != nil {
			return nil, err
		}
	}
	for _, c := range []rules.Controllability{rules.Controllable, rules.PartialControl, rules.NoControl} {
		if err := positiveWeight("controllability", string(c), weights.Controllability[c]); err != nil {
			return nil, err
		}
	}
	return &Engine{weights: weights}, nil
}

// positiveWeight — erreur si la pondération d'une dimension donnée n'est pas
// strictement positive.
func positiveWeight(dimension, name string, w float64) error {
	if w <= 0 {
		return fmt.Errorf("scorer : poids %s %q invalide (%g) : strictement positif requis", dimension, name, w)
	}
	return nil
}

// Score — retourne les findings scorés, triés par priorité décroissante
// (ordre stable à priorité égale). Un finding dont l'effort ou la
// contrôlabilité serait absent du jeu de pondérations obtient une priorité
// nulle — cas impossible en pratique : le loader de règles valide les enums.
func (e *Engine) Score(findings []rules.Finding) []Scored {
	scored := make([]Scored, len(findings))
	for i, f := range findings {
		priority := maxPriority * f.Evidence.TimeShare *
			e.weights.Effort[f.Effort] * e.weights.Controllability[f.Controllability]
		scored[i] = Scored{Finding: f, Priority: priority}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Priority > scored[j].Priority
	})
	return scored
}
