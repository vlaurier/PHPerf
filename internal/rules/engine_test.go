package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phperf/phperf/internal/analyzer"
	"github.com/phperf/phperf/internal/collector"
	"github.com/phperf/phperf/internal/rules"
)

const mb = 1024 * 1024

// entry — sucre pour construire une entrée XHProf lisible dans les tests
// (pmu non exercé ici, il vaut toujours zéro).
func entry(ct, wt, cpu, mu int64) collector.Entry {
	return collector.Entry{CT: ct, WT: wt, CPU: cpu, MU: mu}
}

// normalize — construit un CallGraph valide pour les tests du moteur.
func normalize(t *testing.T, raw collector.RawProfile) *analyzer.CallGraph {
	t.Helper()
	graph, err := analyzer.NewXHProfNormalizer().Normalize(raw)
	require.NoError(t, err)
	return graph
}

// mustEngine — construit un moteur dont les patterns sont valides par construction.
func mustEngine(t *testing.T, rs ...rules.Rule) *rules.Engine {
	t.Helper()
	e, err := rules.NewEngine(rs)
	require.NoError(t, err)
	return e
}

// rule — socle de règle valide avec le matcher donné.
func rule(id string, m rules.Match) rules.Rule {
	return rules.Rule{
		ID:              id,
		Name:            id,
		Description:     "description de test",
		Severity:        rules.SeverityHigh,
		Effort:          rules.EffortLow,
		Controllability: rules.PartialControl,
		Recommendation:  "recommandation de test",
		Match:           m,
	}
}

func TestEvaluate_FunctionPattern(t *testing.T) {
	raw := collector.RawProfile{
		analyzer.RootName:                      entry(1, 1000, 900, 0),
		"main()==>App\\Kernel::boot":           entry(1, 600, 550, 0),
		"App\\Kernel::boot==>App\\Db::connect": entry(1, 400, 380, 0),
	}

	eng := mustEngine(t, rule("db-slow", rules.Match{FunctionPattern: `^App\\Db`}))

	findings, err := eng.Evaluate(normalize(t, raw))
	require.NoError(t, err)
	require.Len(t, findings, 1)

	f := findings[0]
	assert.Equal(t, "db-slow|App\\Db::connect", f.Key)
	assert.Equal(t, "db-slow", f.RuleID)
	assert.Equal(t, "App\\Db::connect", f.Function)
	assert.Equal(t, "App\\Kernel::boot", f.Caller)
	assert.Equal(t, rules.SeverityHigh, f.Severity)
	assert.Equal(t, rules.EffortLow, f.Effort)
	assert.Equal(t, rules.PartialControl, f.Controllability)
	assert.Equal(t, "recommandation de test", f.Recommendation)
	assert.Equal(t, int64(1), f.Evidence.CallCount)
	assert.Equal(t, 0.0, f.Evidence.MemPerCallMB)
	assert.InDelta(t, 0.4, f.Evidence.TimeShare, 0.001) // 400 µs / 1000 µs
}

func TestEvaluate_NoMatchAndRootExcluded(t *testing.T) {
	raw := collector.RawProfile{
		analyzer.RootName: entry(9, 900, 800, 0),
		"main()==>strlen": entry(5, 10, 8, 0),
		"main()==>main()": entry(1, 1, 1, 0), // arête auto-référente sur la racine
	}

	eng := mustEngine(t,
		rule("no-hit", rules.Match{FunctionPattern: `^str_replace$`}),
		rule("root-only", rules.Match{FunctionPattern: `^main\(\)$`}),
	)

	findings, err := eng.Evaluate(normalize(t, raw))
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestEvaluate_CallCountIsEdgeScoped(t *testing.T) {
	raw := collector.RawProfile{
		analyzer.RootName:         entry(1, 9000, 8000, 0),
		"main()==>RepoA::all":     entry(1, 5000, 4000, 0),
		"main()==>RepoB::one":     entry(1, 3000, 2500, 0),
		"RepoA::all==>PDO::query": entry(50, 4000, 3000, 0),
		"RepoB::one==>PDO::query": entry(2, 100, 80, 0),
		"RepoA::all==>log":        entry(2, 50, 40, 0),
	}

	th := rules.Threshold(3)
	eng := mustEngine(t, rule("n-plus-one", rules.Match{CallCountThreshold: &th}))

	findings, err := eng.Evaluate(normalize(t, raw))
	require.NoError(t, err)
	require.Len(t, findings, 1) // log CT=2 < seuil ; PDO::query matche via RepoA

	f := findings[0]
	assert.Equal(t, "PDO::query", f.Function)
	assert.Equal(t, "RepoA::all", f.Caller)
	assert.Equal(t, int64(50), f.Evidence.CallCount)
}

func TestEvaluate_DedupKeepsDominantCaller(t *testing.T) {
	raw := collector.RawProfile{
		analyzer.RootName:         entry(1, 9000, 8000, 0),
		"main()==>RepoA::all":     entry(1, 5000, 4000, 0),
		"main()==>RepoB::all":     entry(1, 3000, 2500, 0),
		"RepoA::all==>PDO::query": entry(10, 2000, 1500, 0),
		"RepoB::all==>PDO::query": entry(20, 2500, 2000, 0),
	}

	th := rules.Threshold(3)
	eng := mustEngine(t, rule("dup-call", rules.Match{CallCountThreshold: &th}))

	findings, err := eng.Evaluate(normalize(t, raw))
	require.NoError(t, err)
	require.Len(t, findings, 1) // dédoublonné par clé stable

	f := findings[0]
	assert.Equal(t, "dup-call|PDO::query", f.Key)
	assert.Equal(t, "RepoB::all", f.Caller) // site dominant : CT=20 > CT=10
	assert.Equal(t, int64(20), f.Evidence.CallCount)
}

func TestEvaluate_MemoryPerCallAggregated(t *testing.T) {
	raw := collector.RawProfile{
		analyzer.RootName: entry(1, 9000, 8000, 0),
		"main()==>a":      entry(1, 100, 90, 0),
		"main()==>b":      entry(1, 100, 90, 0),
		"a==>heavy":       entry(1, 100, 90, 15*mb),
		"b==>heavy":       entry(3, 100, 90, 30*mb),
	}
	// ΣMU = 45 Mo sur ΣCT = 4 appels → 11,25 Mo/appel.

	thPass := rules.Threshold(11.25)
	thFail := rules.Threshold(12)
	eng := mustEngine(t,
		rule("mem-ok", rules.Match{MemoryPerCallThresholdMB: &thPass}),
		rule("mem-too-high", rules.Match{MemoryPerCallThresholdMB: &thFail}),
	)

	findings, err := eng.Evaluate(normalize(t, raw))
	require.NoError(t, err)
	require.Len(t, findings, 1)

	assert.Equal(t, "mem-ok|heavy", findings[0].Key)
	assert.InDelta(t, 11.25, findings[0].Evidence.MemPerCallMB, 0.001)
}

func TestEvaluate_ZeroRootWallTime(t *testing.T) {
	raw := collector.RawProfile{
		analyzer.RootName:     entry(1, 0, 0, 0),
		"main()==>PDO::query": entry(5, 100, 90, 0),
	}

	th := rules.Threshold(3)
	eng := mustEngine(t, rule("n-plus-one", rules.Match{CallCountThreshold: &th}))

	findings, err := eng.Evaluate(normalize(t, raw))
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, 0.0, findings[0].Evidence.TimeShare) // pas de division par zéro
}

func TestEvaluate_RuleOrderDeterministic(t *testing.T) {
	raw := collector.RawProfile{
		analyzer.RootName:     entry(1, 9000, 8000, 0),
		"main()==>PDO::query": entry(5, 100, 90, 0),
	}

	th := rules.Threshold(3)
	eng := mustEngine(t,
		rule("a-rule", rules.Match{CallCountThreshold: &th}),
		rule("b-rule", rules.Match{CallCountThreshold: &th}),
	)

	findings, err := eng.Evaluate(normalize(t, raw))
	require.NoError(t, err)
	require.Len(t, findings, 2)

	keys := []string{findings[0].Key, findings[1].Key}
	assert.Equal(t, []string{"a-rule|PDO::query", "b-rule|PDO::query"}, keys)
}

func TestNewEngine_InvalidPattern(t *testing.T) {
	e, err := rules.NewEngine([]rules.Rule{rule("bad", rules.Match{FunctionPattern: "["})})
	require.Nil(t, e)
	require.ErrorContains(t, err, `"bad"`)
}

func TestNewEngine_InvalidExcludePattern(t *testing.T) {
	th := rules.Threshold(1)
	e, err := rules.NewEngine([]rules.Rule{rule("bad", rules.Match{
		CallCountThreshold: &th,
		ExcludePattern:     "(",
	})})
	require.Nil(t, e)
	require.ErrorContains(t, err, `"bad"`)
}

func TestEvaluate_NilGraph(t *testing.T) {
	eng := mustEngine(t, rule("x", rules.Match{}))

	got, err := eng.Evaluate(nil)
	require.Nil(t, got)
	require.ErrorContains(t, err, "graphe nil")
}

func TestEvaluate_InclusiveExclusiveTimeThresholds(t *testing.T) {
	raw := collector.RawProfile{
		analyzer.RootName: entry(1, 500000, 0, 0),
		"main()==>filler": entry(1, 380000, 0, 0),
		"main()==>A":      entry(1, 80000, 0, 0),
		"A==>sub":         entry(1, 1000, 0, 0),
		"main()==>B":      entry(1, 30000, 0, 0),
		"B==>C":           entry(1, 25000, 0, 0),
	}
	// A : inclusif 80 ms / exclusif 79 ms / part 16 %.
	// C : inclusif = exclusif 25 ms, part 5 %. filler : inclusif 380 ms, part 76 %.

	thInclHit := rules.Threshold(50)   // A : 80 ms ≥ 50
	thExclMiss := rules.Threshold(50)  // C : 25 ms < 50
	thShare := rules.Threshold(25)     // filler : 76 % ≥ 25
	thInclMiss := rules.Threshold(100) // A : 80 ms < 100

	eng := mustEngine(t,
		rule("incl-hit", rules.Match{FunctionPattern: `^A$`, InclusiveWTMsThreshold: &thInclHit}),
		rule("excl-miss", rules.Match{FunctionPattern: `^C$`, ExclusiveWTMsThreshold: &thExclMiss}),
		rule("share-hit", rules.Match{FunctionPattern: `^filler$`, TimeSharePercentThreshold: &thShare}),
		rule("combo-miss", rules.Match{FunctionPattern: `^A$`, InclusiveWTMsThreshold: &thInclMiss}),
	)

	findings, err := eng.Evaluate(normalize(t, raw))
	require.NoError(t, err)

	keys := make([]string, 0, len(findings))
	for _, f := range findings {
		keys = append(keys, f.Key)
	}
	assert.Equal(t, []string{"incl-hit|A", "share-hit|filler"}, keys)
}

func TestEvaluate_CallerCountThreshold(t *testing.T) {
	raw := collector.RawProfile{
		analyzer.RootName: entry(1, 60000, 0, 0),
		"main()==>x":      entry(1, 20000, 0, 0),
		"main()==>y":      entry(1, 20000, 0, 0),
		"main()==>z":      entry(1, 20000, 0, 0),
		"x==>D":           entry(1, 5000, 0, 0),
		"y==>D":           entry(2, 10000, 0, 0),
		"z==>D":           entry(3, 15000, 0, 0),
	} // D : 3 sites d'appel distincts.

	thOK := rules.Threshold(3)
	thHigh := rules.Threshold(4)
	eng := mustEngine(t,
		rule("fanout-ok", rules.Match{CallerCountThreshold: &thOK}),
		rule("fanout-high", rules.Match{CallerCountThreshold: &thHigh}),
	)

	findings, err := eng.Evaluate(normalize(t, raw))
	require.NoError(t, err)

	keys := make([]string, 0, len(findings))
	for _, f := range findings {
		keys = append(keys, f.Key)
	}
	assert.Equal(t, []string{"fanout-ok|D"}, keys)
}

func TestEvaluate_PeakMemoryPerCallAggregated(t *testing.T) {
	raw := collector.RawProfile{
		analyzer.RootName: entry(1, 9000, 0, 0),
		"main()==>a":      entry(1, 4000, 0, 0),
		"main()==>b":      entry(1, 4000, 0, 0),
		"a==>E":           {CT: 1, WT: 3000, PMU: 40 * mb},
		"b==>E":           {CT: 3, WT: 3000, PMU: 120 * mb},
	}
	// ΣPMU = 160 Mo sur ΣCT = 4 appels → pic de 40 Mo/appel.

	thOK := rules.Threshold(35)
	thHigh := rules.Threshold(45)
	eng := mustEngine(t,
		rule("peak-ok", rules.Match{PeakMemPerCallThresholdMB: &thOK}),
		rule("peak-high", rules.Match{PeakMemPerCallThresholdMB: &thHigh}),
	)

	findings, err := eng.Evaluate(normalize(t, raw))
	require.NoError(t, err)
	require.Len(t, findings, 1)

	assert.Equal(t, "peak-ok|E", findings[0].Key)
	assert.InDelta(t, 66.67, findings[0].Evidence.TimeShare*100, 0.01) // 6000 µs (ΣWT) / 9000 µs
}

func TestEvaluate_ExcludePatternShieldsCallee(t *testing.T) {
	raw := collector.RawProfile{
		analyzer.RootName:     entry(1, 340000, 0, 0),
		"main()==>PDO::query": entry(1, 150000, 0, 0),
		"main()==>App\\Calc":  entry(1, 180000, 0, 0),
	}
	// Les deux fonctions dépassent 50 ms d'exclusif ; seule App\Calc doit
	// survivre à l'exclusion des familles I/O.

	th := rules.Threshold(50)
	eng := mustEngine(t, rule("cpu-hot", rules.Match{
		ExclusiveWTMsThreshold: &th,
		ExcludePattern:         `^PDO`,
	}))

	findings, err := eng.Evaluate(normalize(t, raw))
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, "cpu-hot|App\\Calc", findings[0].Key)
}

// loadCatalog — le catalogue d'exemple complet (proto/rules.example.yaml),
// chargé avec les mêmes contraintes strictes qu'en production.
func loadCatalog(t *testing.T) []rules.Rule {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "proto", "rules.example.yaml"))
	require.NoError(t, err)
	rs, err := rules.Load(data)
	require.NoError(t, err)
	return rs
}

// readFixtureJSON — lit un profil XHProf de scripts/fixtures/.
func readFixtureJSON(t *testing.T, name string) collector.RawProfile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "fixtures", name))
	require.NoError(t, err)
	raw, err := collector.DecodeRaw(data)
	require.NoError(t, err)
	return raw
}

// TestCatalogAgainstFixtures — conformité du catalogue complet : chaque
// fixture documente ce que les règles DOIVENT et NE DOIVENT PAS produire
// (listes exactes, ordre déterministe règle puis arête).
func TestCatalogAgainstFixtures(t *testing.T) {
	catalog := loadCatalog(t)
	eng, err := rules.NewEngine(catalog)
	require.NoError(t, err)

	tests := []struct {
		name    string
		fixture string
		want    []string
	}{
		{
			name:    "N+1 classique",
			fixture: "nplus1.json",
			want: []string{
				"n-plus-one-query|PDO::query",
				"dominant-subtree|PDO::query",            // 88,9 % de la trace
				"dominant-subtree|App\\Controller::list", // 97,8 %
				"duplicated-calculation|PDO::query",
			},
		},
		{
			name:    "trace linéaire sans anti-pattern",
			fixture: "linear.json",
			want: []string{
				"dominant-subtree|App\\Db::connect",  // 40 %
				"dominant-subtree|App\\Kernel::boot", // 60 %
			},
		},
		{
			name:    "récursion fib",
			fixture: "recursive.json",
			want: []string{
				"dominant-subtree|App\\Fib::compute",       // 95 %
				"duplicated-calculation|App\\Fib::compute", // 6 appels auto-récursifs
			},
		},
		{
			name:    "hotspot CPU pur",
			fixture: "hotspot.json",
			want: []string{
				"cpu-bound-function|App\\Search::rank", // 65 ms propres
				"dominant-subtree|App\\Search::rank",   // 70 %
			},
		},
		{
			name:    "I/O lourdes : requête lente, fichiers, cache, mail",
			fixture: "io-heavy.json",
			want: []string{
				"slow-single-query|PDO::query",
				"filesystem-io-in-loop|file_get_contents",
				"cache-chatter-in-loop|Redis::get",
				"mail-send-in-loop|mail",
				"cpu-bound-function|App\\Report::build", // le code lui-même, pas ses I/O
				"dominant-subtree|PDO::query",
				"dominant-subtree|App\\Report::build",
				"duplicated-calculation|Redis::get",
				"duplicated-calculation|file_get_contents",
				"duplicated-calculation|mail",
			},
		},
		{
			name:    "code smells détectables au profil",
			fixture: "smells.json",
			want: []string{
				"dominant-subtree|password_verify", // 40 %
				"dominant-subtree|sleep",           // 25 % exactement
				"dominant-subtree|App\\Handler::run",
				"debug-leftovers|var_dump",
				"blocking-sleep|sleep",
				"array-merge-in-loop|array_merge",
				"linear-scan-in-loop|in_array",
				"slow-hash-repeated|password_verify",
				"duplicated-calculation|array_merge",
				"duplicated-calculation|in_array",
				"duplicated-calculation|password_verify",
			},
		},
		{
			name:    "pic mémoire transitoire",
			fixture: "spike.json",
			want: []string{
				"dominant-subtree|gzdecode",
				"dominant-subtree|App\\Import::load",
				"memory-spike-transient|gzdecode", // ~44,8 Mo de pic par appel
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := eng.Evaluate(normalize(t, readFixtureJSON(t, tt.fixture)))
			require.NoError(t, err)

			keys := make([]string, 0, len(findings))
			for _, f := range findings {
				keys = append(keys, f.Key)
			}
			assert.Equal(t, tt.want, keys)
		})
	}
}
