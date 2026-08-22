// Package rules évalue des règles déclaratives (YAML) contre un call graph
// normalisé et produit des findings priorisables.
//
// Le format est décrit par proto/rules.schema.json ; l'exemple de référence
// est proto/rules.example.yaml.
package rules

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Severity — gravité du problème détecté.
type Severity string

// Niveaux de gravité, du plus au moins critique.
const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// Effort — effort estimé pour corriger le problème.
type Effort string

// Niveaux d'effort estimé.
const (
	EffortLow    Effort = "low"
	EffortMedium Effort = "medium"
	EffortHigh   Effort = "high"
)

// Controllability — maîtrise qu'a l'équipe sur la cause du problème.
type Controllability string

// Niveaux de contrôlabilité.
const (
	Controllable   Controllability = "controllable"
	PartialControl Controllability = "partial"
	NoControl      Controllability = "none"
)

// Rule — règle déclarative détectant un anti-pattern dans un call graph.
type Rule struct {
	ID              string          `yaml:"id"`
	Name            string          `yaml:"name"`
	Description     string          `yaml:"description"`
	Severity        Severity        `yaml:"severity"`
	Effort          Effort          `yaml:"effort"`
	Controllability Controllability `yaml:"controllability"`
	Match           Match           `yaml:"match"`
	Recommendation  string          `yaml:"recommendation"`
}

// Match — critères de déclenchement de la règle. Au moins un critère est
// requis ; les critères définis se combinent en ET.
type Match struct {
	FunctionPattern          string     `yaml:"function_pattern,omitempty"`             // regexp sur le callee
	CallCountThreshold       *Threshold `yaml:"call_count_threshold,omitempty"`         // appels caller→callee ≥ seuil
	MemoryPerCallThresholdMB *Threshold `yaml:"memory_per_call_threshold_mb,omitempty"` // Mo/appel ≥ seuil
}

// Threshold — seuil au format ">=N" ou ">=N.N". Seul opérateur supporté en
// v1 : la règle matche quand la métrique observée atteint ou dépasse N.
type Threshold float64

// UnmarshalYAML — parse une valeur YAML ">=N" en Threshold.
func (t *Threshold) UnmarshalYAML(node *yaml.Node) error {
	const op = ">="

	s := strings.TrimSpace(node.Value)
	if !strings.HasPrefix(s, op) {
		return fmt.Errorf("rules : seuil %q invalide (format attendu : %q)", s, op+"N")
	}
	v, err := strconv.ParseFloat(strings.TrimPrefix(s, op), 64)
	if err != nil || v < 0 {
		return fmt.Errorf("rules : seuil %q invalide (nombre positif attendu après %q)", s, op)
	}
	*t = Threshold(v)
	return nil
}
