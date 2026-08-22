package rules_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phperf/phperf/internal/rules"
)

func TestLoad_Valid(t *testing.T) {
	data := []byte(`
rules:
  - id: n-plus-one-query
    name: Requête N+1
    description: Une requête SQL exécutée en boucle.
    severity: critical
    effort: medium
    controllability: controllable
    match:
      function_pattern: "^PDO"
      call_count_threshold: ">=3"
      memory_per_call_threshold_mb: ">=1.5"
    recommendation: Utilisez un fetch batch.
`)
	got, err := rules.Load(data)
	require.NoError(t, err)
	require.Len(t, got, 1)

	r := got[0]
	assert.Equal(t, "n-plus-one-query", r.ID)
	assert.Equal(t, "Requête N+1", r.Name)
	assert.Equal(t, rules.SeverityCritical, r.Severity)
	assert.Equal(t, rules.EffortMedium, r.Effort)
	assert.Equal(t, rules.Controllable, r.Controllability)
	assert.Equal(t, "^PDO", r.Match.FunctionPattern)
	require.NotNil(t, r.Match.CallCountThreshold)
	assert.Equal(t, rules.Threshold(3), *r.Match.CallCountThreshold)
	require.NotNil(t, r.Match.MemoryPerCallThresholdMB)
	assert.InDelta(t, 1.5, float64(*r.Match.MemoryPerCallThresholdMB), 0.0001)
	assert.Equal(t, "Utilisez un fetch batch.", r.Recommendation)
}

func TestLoad_Errors(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr string
	}{
		{
			name:    "fichier vide",
			data:    "",
			wantErr: "fichier vide",
		},
		{
			name:    "aucune règle",
			data:    "rules: []\n",
			wantErr: "aucune règle définie",
		},
		{
			name: "champ inconnu",
			data: `rules:
  - {id: ok, name: n, description: d, severity: low, effort: low, controllability: none,
     match: {function_pattern: "^x"}, typo_field: vrai}
`,
			wantErr: "typo_field",
		},
		{
			name: "id invalide",
			data: `rules:
  - {id: Bad_ID, name: n, description: d, severity: low, effort: low,
     controllability: none, match: {function_pattern: "^x"}}
`,
			wantErr: `id "Bad_ID" invalide`,
		},
		{
			name: "name manquant",
			data: `rules:
  - {id: ok, description: d, severity: low, effort: low,
     controllability: none, match: {function_pattern: "^x"}}
`,
			wantErr: "name manquant",
		},
		{
			name: "description manquante",
			data: `rules:
  - {id: ok, name: n, severity: low, effort: low,
     controllability: none, match: {function_pattern: "^x"}}
`,
			wantErr: "description manquante",
		},
		{
			name: "severity hors enum",
			data: `rules:
  - {id: ok, name: n, description: d, severity: extreme, effort: low,
     controllability: none, match: {function_pattern: "^x"}}
`,
			wantErr: `severity : "extreme" invalide`,
		},
		{
			name: "effort hors enum",
			data: `rules:
  - {id: ok, name: n, description: d, severity: low, effort: huge,
     controllability: none, match: {function_pattern: "^x"}}
`,
			wantErr: `effort : "huge" invalide`,
		},
		{
			name: "controllability hors enum",
			data: `rules:
  - {id: ok, name: n, description: d, severity: low, effort: low,
     controllability: peut-être, match: {function_pattern: "^x"}}
`,
			wantErr: `controllability : "peut-être" invalide`,
		},
		{
			name: "aucun critère de match",
			data: `rules:
  - {id: ok, name: n, description: d, severity: low, effort: low,
     controllability: none, match: {}}
`,
			wantErr: "au moins un critère requis",
		},
		{
			name: "regexp invalide",
			data: `rules:
  - {id: ok, name: n, description: d, severity: low, effort: low,
     controllability: none, match: {function_pattern: "["}}
`,
			wantErr: "function_pattern",
		},
		{
			name: "seuil sans opérateur",
			data: `rules:
  - {id: ok, name: n, description: d, severity: low, effort: low,
     controllability: none, match: {call_count_threshold: "3"}}
`,
			wantErr: `seuil "3" invalide`,
		},
		{
			name: "seuil non numérique",
			data: `rules:
  - {id: ok, name: n, description: d, severity: low, effort: low,
     controllability: none, match: {call_count_threshold: ">=abc"}}
`,
			wantErr: "nombre positif attendu",
		},
		{
			name: "seuil négatif",
			data: `rules:
  - {id: ok, name: n, description: d, severity: low, effort: low,
     controllability: none, match: {memory_per_call_threshold_mb: ">=-1"}}
`,
			wantErr: "nombre positif attendu",
		},
		{
			name: "id dupliqué",
			data: `rules:
  - {id: doublon, name: n, description: d, severity: low, effort: low,
     controllability: none, match: {call_count_threshold: ">=1"}}
  - {id: doublon, name: n2, description: d2, severity: high, effort: high,
     controllability: partial, match: {call_count_threshold: ">=2"}}
`,
			wantErr: "id dupliqué",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rules.Load([]byte(tt.data))
			require.Nil(t, got)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
