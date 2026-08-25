// Command phperf démarre l'interface web de PHPerf : liste des findings
// priorisés et triage (masquage persisté en SQLite).
//
// Le profil est chargé une fois au démarrage ; les décisions de masquage
// survivent aux relances via la base --db. Pour la CI, préférer phperf-ci.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/phperf/phperf/internal/analyzer"
	"github.com/phperf/phperf/internal/collector"
	"github.com/phperf/phperf/internal/report"
	"github.com/phperf/phperf/internal/rules"
	"github.com/phperf/phperf/internal/scorer"
	"github.com/phperf/phperf/internal/storage"
	"github.com/phperf/phperf/internal/ui"
)

// version — tampon d'identification du binaire, écrasé à la compilation
// par `-ldflags "-X main.version=…"` (make build / scripts/release.sh).
var version = "dev"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run — wiring complet ; retourne au lieu de quitter pour laisser les
// defer s'exécuter (fermeture propre de la base).
func run() error {
	var (
		profilePath = flag.String("profile", "", "profil XHProf sérialisé en JSON")
		rulesPath   = flag.String("rules", "", "règles YAML (modèle : proto/rules.example.yaml)")
		scoringPath = flag.String("scoring", "", "pondérations YAML optionnelles (défauts embarqués sinon)")
		dbPath      = flag.String("db", ".phperf.db", "base SQLite des décisions de triage")
		addr        = flag.String("addr", ":8080", "adresse d'écoute HTTP")
		showVersion = flag.Bool("version", false, "affiche la version puis sort")
	)
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return nil
	}
	if *profilePath == "" || *rulesPath == "" {
		return fmt.Errorf("flags requis : --profile et --rules")
	}

	scored, err := evaluate(*profilePath, *rulesPath, *scoringPath)
	if err != nil {
		return err
	}

	store, err := storage.Open(*dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	srv := ui.NewServer("PHPerf — findings priorisés", toView(scored), store)

	log.Printf("Interface disponible sur http://localhost%s", *addr)
	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		return fmt.Errorf("serveur HTTP : %w", err)
	}
	return nil
}

// evaluate — déroule le pipeline lecture du profil → normalisation →
// règles → scoring (même wiring que phperf-ci ; duplication volontaire :
// les binaires restent indépendants).
func evaluate(profilePath, rulesPath, scoringPath string) ([]scorer.Scored, error) {
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

	weights := scorer.DefaultWeights()
	if scoringPath != "" {
		scoringData, err := os.ReadFile(scoringPath)
		if err != nil {
			return nil, fmt.Errorf("scoring : %w", err)
		}
		if weights, err = scorer.LoadWeights(scoringData); err != nil {
			return nil, err
		}
	}
	scorerEngine, err := scorer.NewEngine(weights)
	if err != nil {
		return nil, err
	}

	return scorerEngine.Score(findings), nil
}

// toView — convertit les findings scorés en vues d'affichage ; l'ordre de
// priorité décroissante produit par Score est conservé.
func toView(scored []scorer.Scored) []report.Finding {
	view := make([]report.Finding, 0, len(scored))
	for _, s := range scored {
		f := s.Finding
		view = append(view, report.Finding{
			Key:             f.Key,
			RuleID:          f.RuleID,
			Function:        f.Function,
			Caller:          f.Caller,
			Severity:        string(f.Severity),
			Effort:          string(f.Effort),
			Controllability: string(f.Controllability),
			Recommendation:  f.Recommendation,
			CallCount:       f.Evidence.CallCount,
			TimeShare:       f.Evidence.TimeShare,
			Priority:        s.Priority,
		})
	}
	return view
}
