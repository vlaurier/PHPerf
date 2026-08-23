// Package ui sert l'interface web de triage : liste des findings priorisés
// avec masquage/démasquage persisté via le Store.
//
// Le package ne dépend que de report (rendu) et d'une abstraction Store
// (définie côté consommateur, implémentée par internal/storage) : les
// findings sont fournis par le wiring de cmd/phperf, déjà convertis en vues
// report.Finding.
package ui

import (
	"net/http"

	"github.com/phperf/phperf/internal/report"
)

// Store — décisions de triage persistées (implémenté par *storage.DB).
type Store interface {
	AddMask(key string) error
	RemoveMask(key string) error
	MaskedKeys() (map[string]struct{}, error)
}

// Server — état du run chargé au démarrage (liste de référence, ordre de
// priorité conservé) et store de triage.
type Server struct {
	title    string
	findings []report.Finding
	store    Store
}

// NewServer — construit le serveur ; findings doit être trié par priorité
// décroissante (l'ordre d'affichage est celui de la tranche).
func NewServer(title string, findings []report.Finding, store Store) *Server {
	return &Server{title: title, findings: findings, store: store}
}

// Handler — routes : GET / (liste), POST /mask et POST /unmask (form :
// key=<clé stable> puis redirection vers /).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleList)
	mux.HandleFunc("/mask", s.handleToggle(s.store.AddMask))
	mux.HandleFunc("/unmask", s.handleToggle(s.store.RemoveMask))
	return mux
}

// handleList — rend la page : findings non masqués par défaut, tous si
// ?show_masked=1.
func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	maskedKeys, err := s.store.MaskedKeys()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	showMasked := r.URL.Query().Get("show_masked") == "1"
	view := make([]report.Finding, 0, len(s.findings))
	for _, f := range s.findings {
		_, isMasked := maskedKeys[f.Key]
		if isMasked && !showMasked {
			continue
		}
		f.Masked = isMasked
		view = append(view, f)
	}

	data := report.Data{
		Title:       s.title,
		Findings:    view,
		MaskedCount: len(maskedKeys),
		ShowMasked:  showMasked,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := report.RenderHTML(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleToggle — fabrique le handler POST d'une action de triage
// (masquage ou démasquage) : clé requise, puis redirection vers la liste.
func (s *Server) handleToggle(action func(key string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "méthode non autorisée", http.StatusMethodNotAllowed)
			return
		}
		key := r.FormValue("key")
		if key == "" {
			http.Error(w, "paramètre « key » requis", http.StatusBadRequest)
			return
		}
		if err := action(key); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}
