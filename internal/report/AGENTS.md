# AGENTS.md — internal/report/

> Instructions spécifiques à `internal/report/` — à lire **en plus** de la racine.

## Responsabilité

**Rendu des findings en vues exportables** : page HTML autonome (template
embarqué) et JSON machine. Moteur de rendu **passif** : le filtrage
(masqués visibles ou non) est du ressort de l'appelant (`ui`).

## Implémentation actuelle

- `Finding` — DTO de vue (identité stable, sévérité/effort/maîtrise,
  recommandation, métriques d'evidence, priorité, état masqué). Le package
  définit **ses propres types** : aucun import métier → matrice depguard
  inchangée ; la conversion se fait dans le wiring de `cmd/*`.
- `Data` — `{Title, Findings, MaskedCount, ShowMasked}`.
- `RenderHTML(w, Data)` — template `templates/list.html` embarqué
  (`go:embed`), échappement XSS natif de `html/template`.
- `JSON(Data)` — JSON indenté pour consommation machine.

## Ce qu'il ne faut PAS

- Aucune logique de scoring/matching, aucune persistance, aucun import
  métier ni `storage` (cf. matrice racine).
- Pas de filtrage ici : `ShowMasked` ne pilote que le lien de bascule du
  template.

## Sorties

- HTML : servi par `internal/ui/` (page unique).
- JSON : disponible pour ingestion CI/CD ultérieure.
- La sortie console de `phperf-ci` vit dans `cmd/phperf-ci/pipeline.go`
  (`printRunReport`) et non ici.

## Tests

- Rendu : contenu, ordre conservé, échappement (`<script>`), états vides,
  erreurs d'écriture (`failWriter`), round-trip JSON.
