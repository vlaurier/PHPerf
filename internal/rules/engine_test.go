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

func TestEvaluate_NilGraph(t *testing.T) {
	eng := mustEngine(t, rule("x", rules.Match{}))

	got, err := eng.Evaluate(nil)
	require.Nil(t, got)
	require.ErrorContains(t, err, "graphe nil")
}

func TestExampleRulesAgainstNPlusOneFixture(t *testing.T) {
	rulesData, err := os.ReadFile(filepath.Join("..", "..", "proto", "rules.example.yaml"))
	require.NoError(t, err)
	rs, err := rules.Load(rulesData)
	require.NoError(t, err)

	profData, err := os.ReadFile(filepath.Join("..", "..", "scripts", "fixtures", "nplus1.json"))
	require.NoError(t, err)
	raw, err := collector.DecodeRaw(profData)
	require.NoError(t, err)

	eng, err := rules.NewEngine(rs)
	require.NoError(t, err)

	findings, err := eng.Evaluate(normalize(t, raw))
	require.NoError(t, err)

	keys := make([]string, 0, len(findings))
	for _, f := range findings {
		keys = append(keys, f.Key)
	}
	// network-call-in-loop : pattern non satisfait ;
	// memory-heavy-allocation : ~0,0015 Mo/appel < 10 Mo.
	assert.Equal(t, []string{
		"duplicated-calculation|PDO::query",
		"n-plus-one-query|PDO::query",
	}, keys)
}
