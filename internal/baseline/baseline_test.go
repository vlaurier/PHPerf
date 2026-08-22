package baseline_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phperf/phperf/internal/baseline"
	"github.com/phperf/phperf/internal/rules"
)

// finding — socle de finding minimal, clé surchargeable.
func finding(key string) rules.Finding {
	return rules.Finding{Key: key, RuleID: "rule-" + key, Function: "f()"}
}

func TestLoad_OK(t *testing.T) {
	data := `{"version": 1, "entries": [{"key": "r|f", "rule_id": "r", "function": "f"}]}`

	b, err := baseline.Load([]byte(data))
	require.NoError(t, err)
	require.Len(t, b.Entries, 1)
	assert.Equal(t, "r|f", b.Entries[0].Key)
	assert.Equal(t, "r", b.Entries[0].RuleID)
	assert.Equal(t, "f", b.Entries[0].Function)
}

func TestLoad_Errors(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "JSON malformé", data: `{"version": 1`},
		{name: "fichier vide", data: ``},
		{name: "champ inconnu", data: `{"version": 1, "bogus": true}`},
		{name: "version absente", data: `{}`},
		{name: "version future", data: `{"version": 99}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := baseline.Load([]byte(tt.data))
			require.Error(t, err)
			assert.Nil(t, b)
		})
	}
}

const canonicalTwoEntries = `{
  "version": 1,
  "entries": [
    {
      "key": "a|x",
      "rule_id": "a",
      "function": "x"
    },
    {
      "key": "b|y",
      "rule_id": "b",
      "function": "y"
    }
  ]
}
`

func TestSave_SortedAndDeterministic(t *testing.T) {
	in := &baseline.Baseline{Version: baseline.Version, Entries: []baseline.Entry{
		{Key: "b|y", RuleID: "b", Function: "y"},
		{Key: "a|x", RuleID: "a", Function: "x"},
	}}

	var first, second bytes.Buffer
	require.NoError(t, baseline.Save(&first, in))
	require.NoError(t, baseline.Save(&second, &baseline.Baseline{Version: baseline.Version, Entries: []baseline.Entry{
		{Key: "a|x", RuleID: "a", Function: "x"},
		{Key: "b|y", RuleID: "b", Function: "y"},
	}}))

	assert.Equal(t, canonicalTwoEntries, first.String())
	assert.Equal(t, first.String(), second.String()) // indépendant de l'ordre d'entrée
}

// failingWriter — échoue à la première écriture (couvre la branche d'erreur
// de Save).
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("écriture refusée") }

func TestSave_WriterError(t *testing.T) {
	in := &baseline.Baseline{Version: baseline.Version, Entries: []baseline.Entry{{Key: "a|x"}}}

	err := baseline.Save(failingWriter{}, in)
	require.ErrorContains(t, err, "écriture refusée")
}

func TestSave_EmptyEntries(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, baseline.Save(&out, &baseline.Baseline{Version: baseline.Version}))
	assert.JSONEq(t, `{"version": 1, "entries": []}`, out.String())
}

func TestSave_RejectsWrongVersion(t *testing.T) {
	err := baseline.Save(io.Discard, &baseline.Baseline{Version: 42})
	require.Error(t, err)
}

func TestRoundTrip_LoadSaveLoad(t *testing.T) {
	data := `{"version": 1, "entries": [
		{"key": "n-plus-one-query|PDO::query", "rule_id": "n-plus-one-query", "function": "PDO::query"},
		{"key": "duplicated-calculation|PDO::query", "rule_id": "duplicated-calculation", "function": "PDO::query"}
	]}`

	b, err := baseline.Load([]byte(data))
	require.NoError(t, err)

	var out bytes.Buffer
	require.NoError(t, baseline.Save(&out, b))

	again, err := baseline.Load(out.Bytes())
	require.NoError(t, err)
	// Contenu préservé et tri par clé appliqué au passage.
	assert.Equal(t, []baseline.Entry{
		{Key: "duplicated-calculation|PDO::query", RuleID: "duplicated-calculation", Function: "PDO::query"},
		{Key: "n-plus-one-query|PDO::query", RuleID: "n-plus-one-query", Function: "PDO::query"},
	}, again.Entries)
}

func TestDiff(t *testing.T) {
	tests := []struct {
		name      string
		findings  []rules.Finding
		baseline  []baseline.Entry
		wantNew   []string
		wantKnown int
	}{
		{
			name:     "baseline vide : tout est nouveau",
			findings: []rules.Finding{finding("a|x"), finding("b|y")},
			wantNew:  []string{"a|x", "b|y"},
		},
		{
			name:      "tout est déjà connu",
			findings:  []rules.Finding{finding("a|x")},
			baseline:  []baseline.Entry{{Key: "a|x"}},
			wantNew:   []string{},
			wantKnown: 1,
		},
		{
			name:      "mixte",
			findings:  []rules.Finding{finding("a|x"), finding("b|y"), finding("c|z")},
			baseline:  []baseline.Entry{{Key: "b|y"}},
			wantNew:   []string{"a|x", "c|z"},
			wantKnown: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := baseline.Diff(tt.findings, &baseline.Baseline{Version: baseline.Version, Entries: tt.baseline})

			keys := make([]string, len(res.New))
			for i, f := range res.New {
				keys[i] = f.Key
			}
			assert.Equal(t, tt.wantNew, keys)
			assert.Equal(t, tt.wantKnown, res.Known)
		})
	}
}
