package scorer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phperf/phperf/internal/rules"
	"github.com/phperf/phperf/internal/scorer"
)

func TestLoadWeights_MergesOntoDefaults(t *testing.T) {
	cfg := `
scoring:
  weights:
    effort:
      high: 0.1
    controllability:
      none: 0.05
`
	w, err := scorer.LoadWeights([]byte(cfg))
	require.NoError(t, err)

	// Surcharges appliquées.
	assert.InDelta(t, 0.1, w.Effort[rules.EffortHigh], 0.000001)
	assert.InDelta(t, 0.05, w.Controllability[rules.NoControl], 0.000001)
	// Défauts conservés pour les clés absentes.
	assert.Equal(t, scorer.DefaultWeights().Effort[rules.EffortLow], w.Effort[rules.EffortLow])
	assert.Equal(t, scorer.DefaultWeights().Effort[rules.EffortMedium], w.Effort[rules.EffortMedium])
	assert.Equal(t, scorer.DefaultWeights().Controllability[rules.PartialControl], w.Controllability[rules.PartialControl])
}

func TestLoadWeights_ExampleFileMatchesDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "proto", "scoring.example.yaml"))
	require.NoError(t, err)

	w, err := scorer.LoadWeights(data)
	require.NoError(t, err)
	assert.Equal(t, scorer.DefaultWeights(), w)
}

func TestLoadWeights_RejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  string
	}{
		{name: "fichier vide", cfg: ``},
		{name: "racine scoring absente", cfg: `autre: {}`},
		{name: "champ inconnu", cfg: "scoring:\n  weights:\n    effort:\n      low: 1\n  bogus: true"},
		{
			name: "clé effort inconnue",
			cfg:  "scoring:\n  weights:\n    effort:\n      trivial: 1.0",
		},
		{
			name: "clé contrôlabilité inconnue",
			cfg:  "scoring:\n  weights:\n    controllability:\n      unknowable: 1.0",
		},
		{
			name: "poids effort nul",
			cfg:  "scoring:\n  weights:\n    effort:\n      low: 0",
		},
		{
			name: "poids contrôlabilité négatif",
			cfg:  "scoring:\n  weights:\n    controllability:\n      partial: -0.5",
		},
		{
			name: "valeur non numérique",
			cfg:  "scoring:\n  weights:\n    effort:\n      low: rapide",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, err := scorer.LoadWeights([]byte(tt.cfg))
			require.Error(t, err)
			assert.Equal(t, scorer.WeightSet{}, w)
		})
	}
}
