package analyzer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phperf/phperf/internal/analyzer"
	"github.com/phperf/phperf/internal/collector"
)

// entry — sucre pour construire une entrée XHProf lisible dans les tests.
func entry(ct, wt, cpu, mu, pmu int64) collector.Entry {
	return collector.Entry{CT: ct, WT: wt, CPU: cpu, MU: mu, PMU: pmu}
}

func TestNormalize_Errors(t *testing.T) {
	norm := analyzer.NewXHProfNormalizer()
	validRoot := entry(1, 1000, 900, 50000, 45000)

	tests := []struct {
		name    string
		raw     collector.RawProfile
		wantErr error
	}{
		{
			name:    "profil nil",
			raw:     nil,
			wantErr: analyzer.ErrMissingRoot,
		},
		{
			name:    "profil vide",
			raw:     collector.RawProfile{},
			wantErr: analyzer.ErrMissingRoot,
		},
		{
			name:    "racine sans appel",
			raw:     collector.RawProfile{analyzer.RootName: entry(0, 1000, 900, 50000, 45000)},
			wantErr: analyzer.ErrMissingRoot,
		},
		{
			name: "entrée isolée non racine",
			raw: collector.RawProfile{
				analyzer.RootName: validRoot,
				"strlen()":        entry(3, 30, 25, 512, 512),
			},
			wantErr: analyzer.ErrInvalidEntry,
		},
		{
			name: "wall time négatif",
			raw: collector.RawProfile{
				analyzer.RootName:        validRoot,
				"main()==>App\\Foo::bar": entry(1, -5, 10, 100, 100),
			},
			wantErr: analyzer.ErrNegativeMetrics,
		},
		{
			name: "nombre d'appels négatif",
			raw: collector.RawProfile{
				analyzer.RootName:        validRoot,
				"main()==>App\\Foo::bar": entry(-1, 5, 10, 100, 100),
			},
			wantErr: analyzer.ErrNegativeMetrics,
		},
		{
			name: "nœud non atteignable depuis la racine",
			raw: collector.RawProfile{
				analyzer.RootName: validRoot,
				"ghost==>hidden":  entry(2, 30, 25, 0, 0),
			},
			wantErr: analyzer.ErrUnreachableNode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph, err := norm.Normalize(tt.raw)
			require.Nil(t, graph)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestNormalize_LinearProfile(t *testing.T) {
	raw := collector.RawProfile{
		analyzer.RootName:                      entry(1, 1000, 900, 50000, 45000),
		"main()==>App\\Kernel::boot":           entry(1, 600, 550, 30000, 28000),
		"App\\Kernel::boot==>App\\Db::connect": entry(1, 400, 380, 20000, 19000),
	}

	graph, err := analyzer.NewXHProfNormalizer().Normalize(raw)
	require.NoError(t, err)

	assert.True(t, graph.Root.IsRoot)
	assert.Equal(t, analyzer.RootName, graph.Root.Name)
	assert.Equal(t, int64(1), graph.Root.CallCount)

	assert.Equal(t, int64(1000), graph.Root.InclusiveWT)
	assert.Equal(t, int64(400), graph.Root.ExclusiveWT)

	boot := graph.Nodes["App\\Kernel::boot"]
	require.NotNil(t, boot)
	assert.Equal(t, int64(600), boot.InclusiveWT)
	assert.Equal(t, int64(200), boot.ExclusiveWT)
	assert.Equal(t, int64(1), boot.CallCount)

	connect := graph.Nodes["App\\Db::connect"]
	require.NotNil(t, connect)
	assert.Equal(t, int64(400), connect.InclusiveWT)
	assert.Equal(t, int64(400), connect.ExclusiveWT)
	assert.Equal(t, int64(1), connect.CallCount)
}

func TestNormalize_MultiParentSumsInclusive(t *testing.T) {
	raw := collector.RawProfile{
		analyzer.RootName: entry(1, 1000, 900, 0, 0),
		"main()==>a":      entry(1, 300, 0, 0, 0),
		"main()==>b":      entry(1, 200, 0, 0, 0),
		"a==>shared":      entry(2, 250, 0, 0, 0),
		"b==>shared":      entry(1, 150, 0, 0, 0),
	}

	graph, err := analyzer.NewXHProfNormalizer().Normalize(raw)
	require.NoError(t, err)

	shared := graph.Nodes["shared"]
	require.NotNil(t, shared)
	assert.Equal(t, int64(400), shared.InclusiveWT)
	assert.Equal(t, int64(400), shared.ExclusiveWT)
	assert.Equal(t, int64(3), shared.CallCount)

	assert.Equal(t, int64(50), graph.Nodes["a"].ExclusiveWT)
	assert.Equal(t, int64(50), graph.Nodes["b"].ExclusiveWT)
	assert.Equal(t, int64(500), graph.Root.ExclusiveWT)
	assert.Len(t, graph.Edges, 4)
}

func TestNormalize_RecursiveCycleNotSubtractedTwice(t *testing.T) {
	raw := collector.RawProfile{
		analyzer.RootName: entry(1, 2000, 1800, 0, 0),
		"main()==>fib":    entry(1, 1900, 1700, 0, 0),
		"fib==>fib":       entry(6, 1200, 1100, 0, 0),
	}

	graph, err := analyzer.NewXHProfNormalizer().Normalize(raw)
	require.NoError(t, err)

	fib := graph.Nodes["fib"]
	require.NotNil(t, fib)
	assert.Equal(t, int64(7), fib.CallCount)
	assert.Equal(t, int64(1900), fib.InclusiveWT)
	assert.Equal(t, int64(1900), fib.ExclusiveWT)
	assert.Equal(t, int64(100), graph.Root.ExclusiveWT)
}

func TestNormalize_EdgesSortedDeterministically(t *testing.T) {
	raw := collector.RawProfile{
		analyzer.RootName: entry(1, 1000, 900, 0, 0),
		"main()==>b":      entry(1, 200, 0, 0, 0),
		"main()==>a":      entry(1, 300, 0, 0, 0),
		"b==>c":           entry(1, 100, 0, 0, 0),
	}

	graph, err := analyzer.NewXHProfNormalizer().Normalize(raw)
	require.NoError(t, err)

	keys := make([]string, 0, len(graph.Edges))
	for _, e := range graph.Edges {
		keys = append(keys, e.Caller+"==>"+e.Callee)
	}
	expected := []string{"b==>c", "main()==>a", "main()==>b"}
	assert.Equal(t, expected, keys)
}

func TestFixturesAreNormalizable(t *testing.T) {
	dir := filepath.Join("..", "..", "scripts", "fixtures")
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.NotEmpty(t, files)

	norm := analyzer.NewXHProfNormalizer()

	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		t.Run(f.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, f.Name()))
			require.NoError(t, err)

			raw, err := collector.DecodeRaw(data)
			require.NoError(t, err)

			graph, err := norm.Normalize(raw)
			require.NoError(t, err)

			assert.True(t, graph.Root.IsRoot)
			assert.NotEmpty(t, graph.Edges)
		})
	}
}
