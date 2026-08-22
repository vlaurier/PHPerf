package rules

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/phperf/phperf/internal/analyzer"
)

const bytesPerMB = 1024 * 1024

// Evaluator — évalue un call graph contre un jeu de règles.
type Evaluator interface {
	Evaluate(graph *analyzer.CallGraph) ([]Finding, error)
}

// Engine — Evaluator appliquant un jeu de règles chargé via Load.
type Engine struct {
	rules    []Rule
	patterns []*regexp.Regexp // parallèle à rules ; nil si la règle n'a pas de pattern
}

// NewEngine — précompile les patterns de chaque règle (échec anticipé).
func NewEngine(rules []Rule) (*Engine, error) {
	patterns := make([]*regexp.Regexp, len(rules))
	for i := range rules {
		p := rules[i].Match.FunctionPattern
		if p == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("rules : règle %q : %w", rules[i].ID, err)
		}
		patterns[i] = re
	}
	return &Engine{rules: rules, patterns: patterns}, nil
}

// Evaluate — retourne les findings des règles satisfaites par le graphe.
// Le résultat est déterministe : ordre des règles (fichier) puis ordre des
// arêtes (triées par clé par l'analyzer).
//
// Sémantique des critères :
//   - function_pattern : regexp sur le callee (la racine est exclue) ;
//   - call_count_threshold : nb d'appels d'un même site caller→callee —
//     heuristique « in-loop » : N répétitions dans une même trace ;
//   - memory_per_call_threshold_mb : mémoire moyenne par appel du callee,
//     agrégée sur tous ses sites d'appel (ΣMU / ΣCT).
//
// Chaque finding embarque en outre Evidence.TimeShare : la part du wall time
// du callee dans la trace, consommée par internal/scorer.
//
// Les findings sont dédupliqués par clé stable (règle|callee) : quand
// plusieurs sites déclenchent la même règle sur le même callee, un seul
// finding est produit et Caller retient le site dominant (plus grand CT).
func (e *Engine) Evaluate(graph *analyzer.CallGraph) ([]Finding, error) {
	if graph == nil {
		return nil, errors.New("rules : graphe nil")
	}

	findings := make([]Finding, 0)
	positions := make(map[string]int) // clé stable → indice dans findings

	for i := range e.rules {
		rule := &e.rules[i]
		pattern := e.patterns[i]

		for _, edge := range graph.Edges {
			callee := edge.Callee
			if callee == analyzer.RootName || !patternMatches(pattern, callee) {
				continue
			}
			if th := rule.Match.CallCountThreshold; th != nil && float64(edge.CT) < float64(*th) {
				continue
			}

			var memMB float64
			if th := rule.Match.MemoryPerCallThresholdMB; th != nil {
				memMB = memPerCallMB(graph, callee)
				if memMB < float64(*th) {
					continue
				}
			}

			timeShare := 0.0
			if graph.Root.InclusiveWT > 0 {
				timeShare = float64(graph.Nodes[callee].InclusiveWT) / float64(graph.Root.InclusiveWT)
			}

			key := findingKey(rule.ID, callee)
			if p, ok := positions[key]; ok {
				if edge.CT > findings[p].Evidence.CallCount { // site dominant
					findings[p].Caller = edge.Caller
					findings[p].Evidence.CallCount = edge.CT
				}
				continue
			}

			finding := Finding{
				Key:             key,
				RuleID:          rule.ID,
				Function:        callee,
				Caller:          edge.Caller,
				Severity:        rule.Severity,
				Effort:          rule.Effort,
				Controllability: rule.Controllability,
				Recommendation:  rule.Recommendation,
				Evidence:        Evidence{CallCount: edge.CT, MemPerCallMB: memMB, TimeShare: timeShare},
			}
			positions[key] = len(findings)
			findings = append(findings, finding)
		}
	}
	return findings, nil
}

// patternMatches — vrai si la règle n'a pas de pattern ou s'il matche name.
func patternMatches(pattern *regexp.Regexp, name string) bool {
	return pattern == nil || pattern.MatchString(name)
}

// memPerCallMB — mémoire moyenne par appel d'un callee, tous sites confondus.
// Le callee est toujours présent dans au moins une arête (l'appelant vient
// d'y correspondre), donc ct ne peut pas être nul ici.
func memPerCallMB(g *analyzer.CallGraph, callee string) float64 {
	var ct, mu int64
	for _, e := range g.Edges {
		if e.Callee == callee {
			ct += e.CT
			mu += e.MU
		}
	}
	return float64(mu) / float64(ct) / bytesPerMB
}
