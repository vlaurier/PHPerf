package scorer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phperf/phperf/internal/rules"
	"github.com/phperf/phperf/internal/scorer"
)

// finding — socle de finding avec valeurs surchargeables par le cas de test.
func finding(key string, timeShare float64, effort rules.Effort, ctrl rules.Controllability) rules.Finding {
	return rules.Finding{
		Key:             key,
		RuleID:          "rule",
		Function:        "App\\Service::run",
		Caller:          "main()",
		Severity:        rules.SeverityHigh,
		Effort:          effort,
		Controllability: ctrl,
		Recommendation:  "recommandation de test",
		Evidence:        rules.Evidence{CallCount: 1, TimeShare: timeShare},
	}
}

// mustScorer — construit un scoreur dont les pondérations sont valides.
func mustScorer(t *testing.T, w scorer.WeightSet) *scorer.Engine {
	t.Helper()
	s, err := scorer.NewEngine(w)
	require.NoError(t, err)
	return s
}

func TestDefaultWeights_CoverAllEnums(t *testing.T) {
	w := scorer.DefaultWeights()

	assert.Equal(t, map[rules.Effort]float64{
		rules.EffortLow:    1.0,
		rules.EffortMedium: 0.75,
		rules.EffortHigh:   0.5,
	}, w.Effort)
	assert.Equal(t, map[rules.Controllability]float64{
		rules.Controllable:   1.0,
		rules.PartialControl: 0.75,
		rules.NoControl:      0.5,
	}, w.Controllability)
}

func TestScore_Formula(t *testing.T) {
	tests := []struct {
		name string
		in   rules.Finding
		want float64
	}{
		{
			name: "facteurs neutres (low × controllable)",
			in:   finding("a", 0.5, rules.EffortLow, rules.Controllable),
			want: 50,
		},
		{
			name: "effort élevé pénalisé",
			in:   finding("a", 0.5, rules.EffortHigh, rules.Controllable),
			want: 25, // 100 × 0,5 × 0,5 × 1
		},
		{
			name: "contrôlabilité nulle pénalisée",
			in:   finding("a", 0.5, rules.EffortLow, rules.NoControl),
			want: 25, // 100 × 0,5 × 1 × 0,5
		},
		{
			name: "tous les poids combinés",
			in:   finding("a", 0.9, rules.EffortMedium, rules.PartialControl),
			want: 50.625, // 100 × 0,9 × 0,75 × 0,75
		},
		{
			name: "part de temps nulle",
			in:   finding("a", 0, rules.EffortLow, rules.Controllable),
			want: 0,
		},
		{
			name: "enum absent des pondérations → priorité nulle",
			in:   finding("a", 0.5, rules.Effort("bogus"), rules.Controllable),
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := mustScorer(t, scorer.DefaultWeights()).Score([]rules.Finding{tt.in})
			require.Len(t, out, 1)
			assert.Equal(t, tt.in.Key, out[0].Finding.Key) // finding intact
			assert.InDelta(t, tt.want, out[0].Priority, 0.000001)
		})
	}
}

func TestScore_SortedDescendingStable(t *testing.T) {
	in := []rules.Finding{
		finding("c", 0.2, rules.EffortLow, rules.Controllable),      // 20
		finding("b", 0.9, rules.EffortMedium, rules.PartialControl), // 50,625
		finding("e", 0.5, rules.EffortLow, rules.Controllable),      // 50 (ex æquo)
		finding("a", 0.5, rules.EffortLow, rules.Controllable),      // 50
		finding("d", 0.4, rules.EffortLow, rules.Controllable),      // 40
	}

	out := mustScorer(t, scorer.DefaultWeights()).Score(in)

	keys := make([]string, len(out))
	for i, s := range out {
		keys[i] = s.Finding.Key
	}
	// Tri desc ; à priorité égale l'ordre d'entrée est conservé : « e »
	// précède « a » dans la tranche d'entrée.
	assert.Equal(t, []string{"b", "e", "a", "d", "c"}, keys)
}

func TestScore_EmptyInput(t *testing.T) {
	out := mustScorer(t, scorer.DefaultWeights()).Score(nil)
	assert.Empty(t, out)
}

func TestNewEngine_RejectsNonPositiveWeights(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(w *scorer.WeightSet)
	}{
		{name: "poids effort nul", mutate: func(w *scorer.WeightSet) { w.Effort[rules.EffortMedium] = 0 }},
		{name: "poids effort négatif", mutate: func(w *scorer.WeightSet) { w.Effort[rules.EffortLow] = -1 }},
		{name: "poids contrôlabilité nul", mutate: func(w *scorer.WeightSet) { w.Controllability[rules.NoControl] = 0 }},
		{name: "poids contrôlabilité négatif", mutate: func(w *scorer.WeightSet) { w.Controllability[rules.Controllable] = -0.5 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := scorer.DefaultWeights()
			tt.mutate(&w)

			s, err := scorer.NewEngine(w)
			require.Nil(t, s)
			require.ErrorContains(t, err, "strictement positif")
		})
	}
}
