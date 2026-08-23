package analyzer

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/phperf/phperf/internal/collector"
)

// RootName — clé de l'entrée racine dans un profil XHProf.
const RootName = "main()"

// edgeSeparator — séparateur XHProf entre parent et enfant.
const edgeSeparator = "==>"

// Erreurs sentinelles du package analyzer.

// ErrMissingRoot — le profil ne contient pas d'entrée racine.
var ErrMissingRoot = errors.New("analyzer: entrée racine absente")

// ErrNegativeMetrics — une entrée porte des métriques négatives.
var ErrNegativeMetrics = errors.New("analyzer: métriques négatives")

// ErrInvalidEntry — une clé d'entrée ne respecte pas le format XHProf.
var ErrInvalidEntry = errors.New("analyzer: entrée invalide")

// ErrUnreachableNode — un nœud n'est atteignable depuis la racine.
var ErrUnreachableNode = errors.New("analyzer: nœud hors du graphe racinaire")

// Normalizer — transforme un profil brut en graphe d'appels normalisé.
type Normalizer interface {
	Normalize(raw collector.RawProfile) (*CallGraph, error)
}

// XHProfNormalizer — Normalizer pour le format XHProf (backing par défaut).
type XHProfNormalizer struct{}

// NewXHProfNormalizer — construit un normaliseur XHProf.
func NewXHProfNormalizer() *XHProfNormalizer {
	return &XHProfNormalizer{}
}

// Normalize — parse les entrées XHProf et construit le CallGraph :
// temps inclusifs cumulés hors cycles de récursion, temps exclusifs
// déduits des enfants. Rejette tout profil dont un nœud n'est pas
// atteignable depuis la racine (données malformées).
func (n *XHProfNormalizer) Normalize(raw collector.RawProfile) (*CallGraph, error) {
	root, ok := rootEntry(raw)
	if !ok {
		return nil, fmt.Errorf("%w : %q introuvable", ErrMissingRoot, RootName)
	}

	graph := &CallGraph{
		Nodes:    make(map[string]*Node),
		Children: make(map[string][]Edge),
	}
	graph.Root = graph.node(RootName)
	graph.Root.IsRoot = true
	graph.Root.CallCount = root.CT
	graph.Root.InclusiveWT = root.WT

	for key, entry := range raw {
		if err := checkMetrics(key, entry); err != nil {
			return nil, err
		}
		caller, callee, isEdge := splitKey(key)
		if isEdge {
			graph.addEdge(caller, callee, entry)
			continue
		}
		if caller != RootName {
			return nil, fmt.Errorf("%w : %q sans relation d'appel", ErrInvalidEntry, key)
		}
	}

	if err := computeMetrics(graph); err != nil {
		return nil, err
	}
	sort.SliceStable(graph.Edges, func(i, j int) bool { return graph.Edges[i].key() < graph.Edges[j].key() })
	return graph, nil
}

// key — clé d'identification stable de l'arête ("parent==>enfant").
func (e Edge) key() string {
	return e.Caller + edgeSeparator + e.Callee
}

// node — récupère ou crée un nœud du graphe.
func (g *CallGraph) node(name string) *Node {
	if n, ok := g.Nodes[name]; ok {
		return n
	}
	n := &Node{Name: name}
	g.Nodes[name] = n
	return n
}

// addEdge — enregistre une arête parent → enfant et cumule le nombre
// d'appels de l'enfant. Les temps sont calculés ensuite par computeMetrics,
// seul à même d'exclure les contributions récursives.
func (g *CallGraph) addEdge(caller, callee string, entry collector.Entry) {
	g.node(caller) // s'assure que le nœud parent existe

	e := Edge{Caller: caller, Callee: callee, CT: entry.CT, WT: entry.WT, CPU: entry.CPU, MU: entry.MU, PMU: entry.PMU}
	g.Children[caller] = append(g.Children[caller], e)
	g.Edges = append(g.Edges, e)

	calleeNode := g.node(callee)
	calleeNode.CallCount += entry.CT
}

// rootEntry — extrait l'entrée racine du profil brut.
func rootEntry(raw collector.RawProfile) (collector.Entry, bool) {
	entry, ok := raw[RootName]
	if !ok || entry.CT <= 0 {
		return collector.Entry{}, false
	}
	return entry, true
}

// splitKey — sépare une clé "parent==>enfant" ; retourne la clé seule
// (isEdge=false) s'il n'y a pas de séparateur.
func splitKey(key string) (caller, callee string, isEdge bool) {
	caller, callee, found := strings.Cut(key, edgeSeparator)
	return caller, callee, found
}

// checkMetrics — rejette toute métrique négative.
func checkMetrics(key string, entry collector.Entry) error {
	if entry.CT < 0 || entry.WT < 0 || entry.CPU < 0 || entry.MU < 0 || entry.PMU < 0 {
		return fmt.Errorf("%w : %q", ErrNegativeMetrics, key)
	}
	return nil
}

// computeMetrics — cumule les temps inclusifs puis déduit les temps
// exclusifs de chaque nœud :
//
//	inclusif = somme des arêtes entrantes non récursives ;
//	exclusif = inclusif − somme des arêtes sortantes non récursives.
//
// Un premier parcours identifie les arêtes « récursives » (celles qui
// referment un cycle : la cible est déjà dans la pile d'appels) — le coût
// qu'elles portent est déjà contenu dans le temps du parent qui referme le
// cycle, les compter reviendrait à le doubler.
func computeMetrics(g *CallGraph) error {
	recursive, err := findRecursiveEdges(g)
	if err != nil {
		return err
	}

	for _, e := range g.Edges {
		if recursive[e.key()] {
			continue
		}
		g.Nodes[e.Callee].InclusiveWT += e.WT
	}

	for _, node := range g.Nodes {
		node.ExclusiveWT = node.InclusiveWT
	}
	for _, e := range g.Edges {
		if recursive[e.key()] {
			continue
		}
		g.Nodes[e.Caller].ExclusiveWT -= e.WT
	}
	return nil
}

// findRecursiveEdges — parcourt le graphe depuis la racine et retourne
// l'ensemble des clés d'arêtes refermant un cycle de récursion. Retourne
// ErrUnreachableNode si un nœud du graphe n'est jamais atteint : données
// malformées qui produiraient des métriques trompeuses.
func findRecursiveEdges(g *CallGraph) (map[string]bool, error) {
	recursive := make(map[string]bool)
	onStack := make(map[string]bool)
	done := make(map[string]bool)

	var visit func(name string)
	visit = func(name string) {
		if done[name] {
			return
		}
		onStack[name] = true

		for _, child := range g.Children[name] {
			if onStack[child.Callee] {
				recursive[child.key()] = true
				continue
			}
			visit(child.Callee)
		}

		delete(onStack, name)
		done[name] = true
	}

	visit(RootName)

	names := make([]string, 0, len(g.Nodes))
	for name := range g.Nodes {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names { // rejet déterministe du premier orphelin
		if !done[name] {
			return nil, fmt.Errorf("%w : %q", ErrUnreachableNode, name)
		}
	}
	return recursive, nil
}
