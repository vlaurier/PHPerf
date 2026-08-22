package main

import (
	"fmt"
	"io"
	"os"

	"github.com/phperf/phperf/internal/analyzer"
	"github.com/phperf/phperf/internal/baseline"
	"github.com/phperf/phperf/internal/collector"
	"github.com/phperf/phperf/internal/rules"
	"github.com/phperf/phperf/internal/scorer"
)

// evaluateProfile — déroule le pipeline lecture du profil → normalisation →
// évaluation des règles. Pur wiring : chaque étage délègue à son package.
func evaluateProfile(profilePath, rulesPath string) ([]rules.Finding, error) {
	profileData, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("profil : %w", err)
	}
	raw, err := collector.DecodeRaw(profileData)
	if err != nil {
		return nil, fmt.Errorf("profil %s : %w", profilePath, err)
	}

	graph, err := analyzer.NewXHProfNormalizer().Normalize(raw)
	if err != nil {
		return nil, fmt.Errorf("analyse de %s : %w", profilePath, err)
	}

	rulesData, err := os.ReadFile(rulesPath)
	if err != nil {
		return nil, fmt.Errorf("règles : %w", err)
	}
	defs, err := rules.Load(rulesData)
	if err != nil {
		return nil, fmt.Errorf("règles %s : %w", rulesPath, err)
	}

	engine, err := rules.NewEngine(defs)
	if err != nil {
		return nil, fmt.Errorf("règles %s : %w", rulesPath, err)
	}

	findings, err := engine.Evaluate(graph)
	if err != nil {
		return nil, fmt.Errorf("évaluation de %s : %w", profilePath, err)
	}
	return findings, nil
}

// baselineFile — construit la structure à sérialiser depuis les findings
// courants (Save se charge du tri déterministe).
func baselineFile(findings []rules.Finding) *baseline.Baseline {
	entries := make([]baseline.Entry, 0, len(findings))
	for _, f := range findings {
		entries = append(entries, baseline.Entry{Key: f.Key, RuleID: f.RuleID, Function: f.Function})
	}
	return &baseline.Baseline{Version: baseline.Version, Entries: entries}
}

// loadBaselineFile — lit et décode le fichier de baseline.
func loadBaselineFile(path string) (*baseline.Baseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("baseline : %w", err)
	}
	return baseline.Load(data)
}

// scoreForDisplay — priorité par clé pour l'affichage ; la config scoring
// optionnelle est validée (une config invalide fait échouer la commande).
func scoreForDisplay(findings []rules.Finding, scoringPath string) (map[string]float64, error) {
	weights := scorer.DefaultWeights()
	if scoringPath != "" {
		data, err := os.ReadFile(scoringPath)
		if err != nil {
			return nil, fmt.Errorf("scoring : %w", err)
		}
		weights, err = scorer.LoadWeights(data)
		if err != nil {
			return nil, err
		}
	}

	engine, err := scorer.NewEngine(weights)
	if err != nil {
		return nil, err
	}

	priorities := make(map[string]float64, len(findings))
	for _, s := range engine.Score(findings) {
		priorities[s.Finding.Key] = s.Priority
	}
	return priorities, nil
}

// printRunReport — rapport texte : les nouveaux findings d'abord (avec
// priorité et recommandation), puis le récapitulatif.
func printRunReport(w io.Writer, res baseline.Result, priorities map[string]float64) {
	for _, f := range res.New {
		_, _ = fmt.Fprintf(w, "\nNOUVEAU  [%s] %s\n", f.RuleID, f.Function)
		_, _ = fmt.Fprintf(w, "  priorité %.1f/100 · %d appel(s) depuis %s\n", priorities[f.Key], f.Evidence.CallCount, f.Caller)
		_, _ = fmt.Fprintf(w, "  → %s\n", f.Recommendation)
	}
	_, _ = fmt.Fprintf(w, "\n%d finding(s) : %d nouveau(x), %d en baseline.\n", len(res.New)+res.Known, len(res.New), res.Known)
}
