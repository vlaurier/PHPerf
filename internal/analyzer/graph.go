// Package analyzer normalise les profils bruts (XHProf) en un call graph
// unifié, consommé par internal/rules et internal/scorer.
package analyzer

// Node — fonction ou méthode observée dans la trace.
type Node struct {
	Name        string
	IsRoot      bool
	CallCount   int64
	InclusiveWT int64 // temps inclusif : le sien + celui de ses enfants
	ExclusiveWT int64 // temps exclusif (self) : déduit des enfants hors récursion
}

// Edge — appel parent → enfant, avec le coût de l'enfant via ce parent.
type Edge struct {
	Caller string
	Callee string
	CT     int64
	WT     int64
	CPU    int64
	MU     int64
}

// CallGraph — graphe d'appels normalisé issu d'un profil brut.
type CallGraph struct {
	Root     *Node
	Nodes    map[string]*Node
	Edges    []Edge
	Children map[string][]Edge // index : nom du parent → arêtes sortantes
}
