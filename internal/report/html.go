package report

import (
	"embed"
	"fmt"
	"html/template"
	"io"
)

//go:embed templates/list.html
var listFS embed.FS

// listTmpl — parsé une seule fois à l'initialisation ; le nom doit
// correspondre au fichier (ParseFS nomme les templates d'après leur base).
// html/template échappe toutes les interpolations (protection XSS par défaut).
var listTmpl = template.Must(template.New("list.html").Funcs(template.FuncMap{
	"mul100": func(v float64) float64 { return v * 100 }, // part de temps → pourcentage
}).ParseFS(listFS, "templates/list.html"))

// RenderHTML — écrit la page de liste des findings dans w.
func RenderHTML(w io.Writer, data Data) error {
	if err := listTmpl.Execute(w, data); err != nil {
		return fmt.Errorf("report : rendu HTML : %w", err)
	}
	return nil
}
