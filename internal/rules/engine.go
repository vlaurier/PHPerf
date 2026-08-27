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
	rules      []Rule
	patterns   []*regexp.Regexp    // parallèle à rules ; nil si la règle n'a pas de pattern
	excludes   []*regexp.Regexp    // parallèle à rules ; nil si la règle n'exclut rien
	supersedes map[string][]string // id → ids supersedés (fermeture transitive)
}

// NewEngine — précompile les patterns de chaque règle et résout la relation
// supersedes (références connues, absence de cycle) : échecs anticipés.
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

	supersedes, err := supersedeClosure(rules)
	if err != nil {
		return nil, err
	}
	return &Engine{rules: rules, patterns: patterns, excludes: excludes, supersedes: supersedes}, nil
}

// supersedeClosure — valide la relation supersedes (références connues,
// absence de cycle) puis calcule sa fermeture transitive : pour chaque id,
// l'ensemble des ids qu'elle remplace directement ou indirectement.
func supersedeClosure(rules []Rule) (map[string][]string, error) {
	index := make(map[string]int, len(rules))
	for i := range rules {
		index[rules[i].ID] = i
	}

	// Détection de cycle et référence inconnue (parcours en profondeur,
	// coloré : un nœud re-rencontré sur le chemin courant = cycle).
	const (
		unvisited = iota
		visiting
		done
	)
	state := make([]uint8, len(rules))
	var visit func(int) error
	visit = func(i int) error {
		if state[i] == done {
			return nil
		}
		state[i] = visiting
		for _, id := range rules[i].Supersedes {
			j, known := index[id]
			if !known {
				return fmt.Errorf("rules : règle %q supersede la règle inconnue %q", rules[i].ID, id)
			}
			if state[j] == visiting {
				return fmt.Errorf("rules : cycle supersedes impliquant la règle %q", rules[i].ID)
			}
			if state[j] == unvisited {
				if err := visit(j); err != nil {
					return err
				}
			}
		}
		state[i] = done
		return nil
	}
	for i := range rules {
		if err := visit(i); err != nil {
			return nil, err
		}
	}

	// Fermeture transitive — le graphe est acyclique, la collection est sûre.
	closure := make(map[string][]string)
	for i := range rules {
		if len(rules[i].Supersedes) == 0 {
			continue
		}
		seen := make(map[string]bool)
		var collect func(id string)
		collect = func(id string) {
			if seen[id] {
				return
			}
			seen[id] = true
			closure[rules[i].ID] = append(closure[rules[i].ID], id)
			for _, next := range rules[index[id]].Supersedes {
				collect(next)
			}
		}
		for _, id := range rules[i].Supersedes {
			collect(id)
		}
	}
	return closure, nil
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
//
// Enfin, la relation supersedes est appliquée : quand une règle R a tiré
// sur un callee, les findings des règles qu'elle supersede (transitivement)
// sur ce même callee sont éliminés — le remède de R rend le leur sans objet.
func (e *Engine) Evaluate(graph *analyzer.CallGraph) ([]Finding, error) {
	if graph == nil {
		return nil, errors.New("rules : graphe nil")
	}

	cache := newStatsCache(graph)
	findings := make([]Finding, 0)
	positions := make(map[string]int)         // clé stable → indice dans findings
	fired := make(map[string]map[string]bool) // id de règle → callees où elle a tiré

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
			if fired[rule.ID] == nil {
				fired[rule.ID] = make(map[string]bool)
			}
			fired[rule.ID][callee] = true

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
	return applySupersedes(findings, e.supersedes, fired), nil
}

// applySupersedes — retire les findings des règles remplacées : quand une
// règle a tiré sur un callee, les findings des règles qu'elle supersede sur
// ce même callee sont éliminés. Le résultat final reste dans l'ordre
// déterministe produit par Evaluate.
func applySupersedes(findings []Finding, supersedes map[string][]string, fired map[string]map[string]bool) []Finding {
	kill := make(map[string]map[string]bool) // callee → règles remplacées
	for ruleID, callees := range fired {
		for callee := range callees {
			for _, sup := range supersedes[ruleID] {
				if kill[callee] == nil {
					kill[callee] = make(map[string]bool)
				}
				kill[callee][sup] = true
			}
		}
	}

	out := findings[:0]
	for _, f := range findings {
		if kill[f.Function][f.RuleID] {
			continue
		}
		out = append(out, f)
	}
	return out
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
