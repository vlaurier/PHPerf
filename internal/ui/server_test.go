package ui_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phperf/phperf/internal/report"
	"github.com/phperf/phperf/internal/ui"
)

// memStore — Store en mémoire pour les tests de handlers.
type memStore struct {
	masks map[string]struct{}
}

func newMemStore(keys ...string) *memStore {
	m := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		m[k] = struct{}{}
	}
	return &memStore{masks: m}
}

func (s *memStore) AddMask(key string) error    { s.masks[key] = struct{}{}; return nil }
func (s *memStore) RemoveMask(key string) error { delete(s.masks, key); return nil }
func (s *memStore) MaskedKeys() (map[string]struct{}, error) {
	out := make(map[string]struct{}, len(s.masks))
	for k := range s.masks {
		out[k] = struct{}{}
	}
	return out, nil
}

// errStore — échoue systématiquement (chemin d'erreur HTTP 500).
type errStore struct{}

func (errStore) AddMask(string) error                     { return assert.AnError }
func (errStore) RemoveMask(string) error                  { return assert.AnError }
func (errStore) MaskedKeys() (map[string]struct{}, error) { return nil, assert.AnError }

func sampleFindings() []report.Finding {
	return []report.Finding{
		{Key: "n-plus-one-query|PDO::query", RuleID: "n-plus-one-query", Function: "PDO::query", Priority: 88.9},
		{Key: "duplicated-calculation|PDO::query", RuleID: "duplicated-calculation", Function: "PDO::query", Priority: 50.6},
	}
}

func TestHandler_ListHidesMaskedByDefault(t *testing.T) {
	srv := ui.NewServer("t", sampleFindings(), newMemStore("duplicated-calculation|PDO::query"))

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "n-plus-one-query")
	assert.NotContains(t, body, "duplicated-calculation") // masqué → absent
	assert.Contains(t, body, "Afficher les 1 finding(s) masqué(s)")
}

func TestHandler_ListShowsMaskedWhenAsked(t *testing.T) {
	srv := ui.NewServer("t", sampleFindings(), newMemStore("duplicated-calculation|PDO::query"))

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?show_masked=1", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `class="masked"`)
	assert.Contains(t, body, `action="/unmask"`)
	assert.Contains(t, body, "Masquer les findings masqués")
}

func TestHandler_MaskThenUnmask(t *testing.T) {
	store := newMemStore()
	srv := ui.NewServer("t", sampleFindings(), store)
	handler := srv.Handler()

	form := url.Values{"key": {"n-plus-one-query|PDO::query"}}
	req := httptest.NewRequest(http.MethodPost, "/mask", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.Len(t, store.masks, 1)

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.NotContains(t, rec.Body.String(), "n-plus-one-query")

	form = url.Values{"key": {"n-plus-one-query|PDO::query"}}
	req = httptest.NewRequest(http.MethodPost, "/unmask", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Empty(t, store.masks)
}

func TestHandler_ToggleValidation(t *testing.T) {
	srv := ui.NewServer("t", sampleFindings(), newMemStore())
	handler := srv.Handler()

	tests := []struct {
		name string
		req  *http.Request
		want int
	}{
		{
			name: "POST /mask sans clé",
			req:  httptest.NewRequest(http.MethodPost, "/mask", strings.NewReader("")),
			want: http.StatusBadRequest,
		},
		{
			name: "GET sur action de triage",
			req:  httptest.NewRequest(http.MethodGet, "/mask?key=x", nil),
			want: http.StatusMethodNotAllowed,
		},
		{
			name: "POST sur la liste",
			req:  httptest.NewRequest(http.MethodPost, "/", strings.NewReader("")),
			want: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, tt.req)
			assert.Equal(t, tt.want, rec.Code)
		})
	}
}

func TestHandler_ToggleStoreErrorReturns500(t *testing.T) {
	srv := ui.NewServer("t", sampleFindings(), errStore{})

	form := url.Values{"key": {"x"}}
	req := httptest.NewRequest(http.MethodPost, "/mask", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_StoreErrorReturns500(t *testing.T) {
	srv := ui.NewServer("t", sampleFindings(), errStore{})

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
