// Command phperf-ci est l'outil d'intégration continue de PHPerf.
//
// Il analyse un profil XHProf sérialisé en JSON via le moteur de règles,
// calcule les scores de priorité puis compare les findings à la baseline
// (approche façon PHPStan) : seuls les findings nouveaux font échouer la CI.
//
// Exit codes:
//
//	0 — aucun nouveau finding par rapport à la baseline
//	1 — nouveaux findings non couverts par la baseline
//	2 — erreur d'exécution ou d'usage
//
// Usage:
//
//	phperf-ci baseline --profile=profil.json --rules=rules.yaml
//	phperf-ci run --profile=profil.json --rules=rules.yaml --baseline=.phperf-baseline.json
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitCode(err))
	}
}
