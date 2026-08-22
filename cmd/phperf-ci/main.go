// Command phperf-ci est l'outil d'intégration continue de PHPerf.
//
// Il exécute un profilage XHProf sur une requête ou un test, analyse le
// résultat via le moteur de règles, calcule les scores de priorité puis
// compare les findings à la baseline (approche façon PHPStan) : seuls
// les findings nouveaux font échouer la CI.
//
// Exit codes:
//
//	0 — aucun nouveau finding par rapport à la baseline
//	1 — nouveaux findings non couverts par la baseline
//	2 — erreur d'exécution (impossible de profiler)
//
// Usage:
//
//	phperf-ci baseline --script=test_request.php
//	phperf-ci run --script=test_request.php --baseline=.phperf-baseline.json
package main

import "fmt"

func main() {
	fmt.Println("phperf-ci — à implémenter")
}
