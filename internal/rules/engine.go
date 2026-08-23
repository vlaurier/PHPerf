package rules

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/phperf/phperf/internal/analyzer"
)

const (
	bytesPerMB  = 1024 * 1024
	microsPerMS = 1000.0
)

// Evaluator — évalue un call graph contre un jeu de règles.
type Evaluator interface {
	Evaluate(graph *analyzer.CallGraph) ([]Finding, error)
}

// Engine — Evaluator appliquant un jeu de règles chargé via Load.
type Engine struct {
	rules    []Rule
	patterns []*regexp.Regexp // parallèle à rules ; nil si la règle n'a pas de pattern
	excludes []*regexp.Regexp // parallèle à rules ; nil si la règle n'exclut rien
}

// NewEngine — précompile les patterns de chaque règle (échec anticipé).
func NewEngine(rules []Rule) (*Engine, error) {
	patterns := make([]*regexp.Regexp, len(rules))
	excludes := make([]*regexp.Regexp, len(rules))
	for i := range rules {
		compile := func(p string) (*regexp.Regexp, error) {
			if p == "" {
				return nil, nil
			}
			re, err := regexp.Compile(p)
			if err != nil {
				return nil, fmt.Errorf("rules : règle %q : %w", rules[i].ID, err)
			}
			return re, nil
		}
		var err error
		if patterns[i], err = compile(rules[i].Match.FunctionPattern); err != nil {
			return nil, err
		}
		if excludes[i], err = compile(rules[i].Match.ExcludePattern); err != nil {
			return nil, err
		}
	}
	return &Engine{rules: rules, patterns: patterns, excludes: excludes}, nil
}

// Evaluate — retourne les findings des règles satisfaites par le graphe.
// Le résultat est déterministe : ordre des règles (fichier) puis ordre des
// arêtes (triées par clé par l'analyzer).
//
// Sémantique des critères (cf. Match) : pattern et call_count s'apprécient
// par site d'appel ; les seuils mémoire/temps/fan-out portent sur l'agrégat
// du callee, tous sites confondus ; exclude_pattern écarte le callee avant
// toute évaluation des seuils.
//
// Les findings sont dédupliqués par clé stable (règle|callee) : quand
// plusieurs sites déclenchent la même règle sur le même callee, un seul
// finding est produit et Caller retient le site dominant (plus grand CT).
func (e *Engine) Evaluate(graph *analyzer.CallGraph) ([]Finding, error) {
	if graph == nil {
		return nil, errors.New("rules : graphe nil")
	}

	cache := newStatsCache(graph)
	findings := make([]Finding, 0)
	positions := make(map[string]int) // clé stable → indice dans findings

	for i := range e.rules {
		rule := &e.rules[i]
		for _, edge := range graph.Edges {
			callee := edge.Callee
			if callee == analyzer.RootName || !patternMatches(e.patterns[i], callee) {
				continue
			}
			if ex := e.excludes[i]; ex != nil && ex.MatchString(callee) {
				continue
			}
			st := cache.get(callee)
			if !rule.Match.satisfied(st, edge.CT) {
				continue
			}

			key := findingKey(rule.ID, callee)
			if p, ok := positions[key]; ok {
				if edge.CT > findings[p].Evidence.CallCount { // site dominant
					findings[p].Caller = edge.Caller
					findings[p].Evidence.CallCount = edge.CT
				}
				continue
			}

			findings = append(findings, Finding{
				Key:             key,
				RuleID:          rule.ID,
				Function:        callee,
				Caller:          edge.Caller,
				Severity:        rule.Severity,
				Effort:          rule.Effort,
				Controllability: rule.Controllability,
				Recommendation:  rule.Recommendation,
				Evidence: Evidence{
					CallCount:    edge.CT,
					MemPerCallMB: st.memPerCallMB,
					TimeShare:    st.timeSharePct / 100,
				},
			})
			positions[key] = len(findings) - 1
		}
	}
	return findings, nil
}

// patternMatches — vrai si la règle n'a pas de pattern ou s'il matche name.
func patternMatches(pattern *regexp.Regexp, name string) bool {
	return pattern == nil || pattern.MatchString(name)
}

// satisfied — évalue tous les critères définis (ET logique) : callCount est
// le nombre d'appels du site courant ; les autres métriques viennent des
// agrégats par callee.
func (m Match) satisfied(st *calleeStats, callCount int64) bool {
	if th := m.CallCountThreshold; th != nil && float64(callCount) < float64(*th) {
		return false
	}
	if th := m.MemoryPerCallThresholdMB; th != nil && st.memPerCallMB < float64(*th) {
		return false
	}
	if th := m.PeakMemPerCallThresholdMB; th != nil && st.peakMemPerCallMB < float64(*th) {
		return false
	}
	if th := m.InclusiveWTMsThreshold; th != nil && st.inclusiveMS < float64(*th) {
		return false
	}
	if th := m.ExclusiveWTMsThreshold; th != nil && st.exclusiveMS < float64(*th) {
		return false
	}
	if th := m.TimeSharePercentThreshold; th != nil && st.timeSharePct < float64(*th) {
		return false
	}
	if th := m.CallerCountThreshold; th != nil && float64(st.callers) < float64(*th) {
		return false
	}
	return true
}

// calleeStats — agrégats d'un callee, exprimés dans les unités des seuils
// YAML (ms, %, Mo) pour un comparaison directe.
type calleeStats struct {
	memPerCallMB     float64 // ΣMU / ΣCT sur tous ses sites d'appel
	peakMemPerCallMB float64 // ΣPMU / ΣCT
	callers          int     // sites d'appel distincts
	inclusiveMS      float64 // temps inclusif du nœud (avec ses enfants)
	exclusiveMS      float64 // temps propre, hors enfants
	timeSharePct     float64 // part du wall time total de la trace, 0–100
}

// statsCache — calcule paresseusement les agrégats d'un callee, une seule
// fois par évaluation (les règles sont nombreuses, les arêtes partagées).
type statsCache struct {
	g     *analyzer.CallGraph
	stats map[string]*calleeStats
}

func newStatsCache(g *analyzer.CallGraph) *statsCache {
	return &statsCache{g: g, stats: map[string]*calleeStats{}}
}

// get — agrégats du callee ; l'appelant a déjà vérifié qu'une arête y mène,
// donc ΣCT ne peut pas être nul ici.
func (c *statsCache) get(callee string) *calleeStats {
	if s, ok := c.stats[callee]; ok {
		return s
	}

	s := &calleeStats{}
	var ct, mu, pmu int64
	for _, e := range c.g.Edges {
		if e.Callee != callee {
			continue
		}
		ct += e.CT
		mu += e.MU
		pmu += e.PMU
		s.callers++
	}
	s.memPerCallMB = float64(mu) / float64(ct) / bytesPerMB
	s.peakMemPerCallMB = float64(pmu) / float64(ct) / bytesPerMB

	node := c.g.Nodes[callee]
	s.inclusiveMS = float64(node.InclusiveWT) / microsPerMS
	s.exclusiveMS = float64(node.ExclusiveWT) / microsPerMS
	if c.g.Root.InclusiveWT > 0 {
		s.timeSharePct = float64(node.InclusiveWT) / float64(c.g.Root.InclusiveWT) * 100
	}

	c.stats[callee] = s
	return s
}
