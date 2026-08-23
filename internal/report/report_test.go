package report_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phperf/phperf/internal/report"
)

// failWriter — writer qui échoue systématiquement.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("écriture impossible") }

func sampleData() report.Data {
	return report.Data{
		Title: "PHPerf — findings",
		Findings: []report.Finding{
			{
				Key:             "n-plus-one-query|PDO::query",
				RuleID:          "n-plus-one-query",
				Function:        "PDO::query",
				Caller:          "App\\Controller::list",
				Severity:        "critical",
				Effort:          "medium",
				Controllability: "controllable",
				Recommendation:  "Utilisez un fetch batch (fetchJoin, eager loading).",
				CallCount:       50,
				TimeShare:       0.4,
				Priority:        88.9,
			},
			{
				Key:      "duplicated-calculation|PDO::query",
				RuleID:   "duplicated-calculation",
				Function: "PDO::query",
				Masked:   true,
				Priority: 53.3,
			},
		},
		MaskedCount: 1,
	}
}

func TestRenderHTML_WriteError(t *testing.T) {
	err := report.RenderHTML(failWriter{}, report.Data{Title: "t"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rendu HTML")
}

func TestJSON_RoundTrip(t *testing.T) {
	data := sampleData()

	out, err := report.JSON(data)
	require.NoError(t, err)

	var got report.Data
	require.NoError(t, json.Unmarshal(out, &got))

	require.Len(t, got.Findings, 2)
	assert.Equal(t, data.Findings[0].Key, got.Findings[0].Key)
	assert.InDelta(t, 88.9, got.Findings[0].Priority, 0.0001)
	assert.Equal(t, int64(50), got.Findings[0].CallCount)
}

func TestRenderHTML_ContentAndOrder(t *testing.T) {
	data := sampleData()
	data.ShowMasked = true // les deux findings visibles

	var buf bytes.Buffer
	require.NoError(t, report.RenderHTML(&buf, data))
	html := buf.String()

	assert.Contains(t, html, "PHPerf — findings")
	assert.Contains(t, html, "n-plus-one-query")
	assert.Contains(t, html, "PDO::query")
	assert.Contains(t, html, "88.9")                  // priorité
	assert.Contains(t, html, "40.0")                  // TimeShare en %
	assert.Contains(t, html, "App\\Controller::list") // backslash préservé (SetEscapeHTML false côté JSON ; template échappe seulement <>&')
	assert.Contains(t, html, `action="/mask"`)
	assert.Contains(t, html, `action="/unmask"`)
	assert.Contains(t, html, "Démasquer")
	assert.Contains(t, html, "Utilisez un fetch batch")
}

// TestRenderHTML_MaskedToggleLink — le filtrage des findings masqués est du
// ressort de l'appelant (ui) ; ici on vérifie uniquement le lien de bascule.
func TestRenderHTML_MaskedToggleLink(t *testing.T) {
	tests := []struct {
		name       string
		showMasked bool
		masked     int
		want       string
		notWant    string
	}{
		{
			name:    "masqués cachés",
			masked:  1,
			want:    "Afficher les 1 finding(s) masqué(s)",
			notWant: "Masquer les findings masqués",
		},
		{
			name:       "masqués affichés",
			showMasked: true,
			masked:     1,
			want:       "Masquer les findings masqués",
			notWant:    "Afficher les 1 finding(s) masqué(s)",
		},
		{
			name:    "aucun masqué",
			masked:  0,
			notWant: "show_masked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, report.RenderHTML(&buf,
				report.Data{Title: "t", MaskedCount: tt.masked, ShowMasked: tt.showMasked}))
			html := buf.String()
			if tt.want != "" {
				assert.Contains(t, html, tt.want)
			}
			assert.NotContains(t, html, tt.notWant)
		})
	}
}

func TestRenderHTML_EscapesUserContent(t *testing.T) {
	data := report.Data{Title: "t", Findings: []report.Finding{{
		Key:      "r|<script>alert(1)</script>",
		Function: "<b>gras</b>",
	}}}

	var buf bytes.Buffer
	require.NoError(t, report.RenderHTML(&buf, data))
	html := buf.String()

	assert.NotContains(t, html, "<script>alert")
	assert.Contains(t, html, "&lt;script&gt;alert(1)&lt;/script&gt;")
	assert.Contains(t, html, "&lt;b&gt;gras&lt;/b&gt;")
}

func TestRenderHTML_EmptyList(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, report.RenderHTML(&buf, report.Data{Title: "t"}))

	assert.Contains(t, buf.String(), "Aucun finding")
}
