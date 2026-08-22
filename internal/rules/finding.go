package rules

// Evidence — métriques observées à l'origine du finding.
type Evidence struct {
	CallCount    int64   // nb d'appels depuis le site d'appel dominant
	MemPerCallMB float64 // mémoire moyenne par appel ; renseigné si la règle porte un seuil mémoire
	TimeShare    float64 // part du wall time du callee dans la trace (0–1) ; consommée par le scoreur
}

// Finding — anti-pattern détecté par une règle sur le call graph.
type Finding struct {
	Key             string // clé stable pour la baseline : "<RuleID>|<fonction>"
	RuleID          string
	Function        string // callee concerné
	Caller          string // site d'appel dominant, contexte informatif
	Severity        Severity
	Effort          Effort
	Controllability Controllability
	Recommendation  string
	Evidence        Evidence
}

// findingKey — construit la clé stable d'un finding : même entrée → même clé,
// indépendante de l'ordre de parcours du graphe.
func findingKey(ruleID, function string) string {
	return ruleID + "|" + function
}
