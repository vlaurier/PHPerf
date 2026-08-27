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
	// Supersedes — ids de règles que cette règle remplace : quand elle tire
	// sur un callee, les findings des règles supersedées sur ce même callee
	// sont retirés (résolution transitive). À déclarer depuis la règle la
	// plus spécifique vers celles qu'elle rend sans objet.
	Supersedes []string `yaml:"supersedes,omitempty"`
}

// Match — critères de déclenchement de la règle. Au moins un critère est
// requis ; les critères définis se combinent en ET.
//
// Deux périmètres d'observation :
//   - par site d'appel : function_pattern (sur le callee) et
//     call_count_threshold (heuristique « in-loop ») ;
//   - par callee, agrégés sur tous ses sites d'appel :
//     memory_per_call_threshold_mb et peak_memory_per_call_threshold_mb
//     (ΣMU|ΣPMU / ΣCT), inclusive_wt_ms_threshold / exclusive_wt_ms_threshold
//     (temps du nœud, l'exclusif étant le temps propre hors enfants),
//     time_share_percent_threshold (part du wall time total de la trace,
//     0–100), caller_count_threshold (nombre de sites distincts).
//
// exclude_pattern (regexp sur le callee) retire des faux positifs : un
// callee qui y correspond est ignoré par la règle, même si tous les autres
// critères passent — typiquement pour réserver une règle de temps aux
// fonctions de calcul et non aux familles d'I/O.
type Match struct {
	FunctionPattern           string     `yaml:"function_pattern,omitempty"`
	ExcludePattern            string     `yaml:"exclude_pattern,omitempty"`
	CallCountThreshold        *Threshold `yaml:"call_count_threshold,omitempty"`
	MemoryPerCallThresholdMB  *Threshold `yaml:"memory_per_call_threshold_mb,omitempty"`      // Mo/appel ≥ seuil
	PeakMemPerCallThresholdMB *Threshold `yaml:"peak_memory_per_call_threshold_mb,omitempty"` // pic Mo/appel ≥ seuil
	InclusiveWTMsThreshold    *Threshold `yaml:"inclusive_wt_ms_threshold,omitempty"`         // temps inclusif ≥ X ms
	ExclusiveWTMsThreshold    *Threshold `yaml:"exclusive_wt_ms_threshold,omitempty"`         // temps propre ≥ X ms
	TimeSharePercentThreshold *Threshold `yaml:"time_share_percent_threshold,omitempty"`      // part de trace ≥ X %
	CallerCountThreshold      *Threshold `yaml:"caller_count_threshold,omitempty"`            // sites distincts ≥ N
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
