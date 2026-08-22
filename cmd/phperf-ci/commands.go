package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/phperf/phperf/internal/baseline"
)

const defaultBaselinePath = ".phperf-baseline.json"

// codedError — erreur portant un code de sortie explicite (cf. exit codes
// documentés dans AGENTS.md).
type codedError struct {
	code int
	msg  string
}

func (e *codedError) Error() string { return e.msg }

// exitCode — code de sortie d'une erreur : code porté si défini, sinon
// erreur runtime générique.
func exitCode(err error) int {
	var ce *codedError
	if errors.As(err, &ce) {
		return ce.code
	}
	return 2
}

// Execute — construit l'arbre de commandes et l'exécute. Les erreurs sont
// affichées par main ; Silence* évite le double affichage cobra.
func Execute() error {
	root := &cobra.Command{
		Use:           "phperf-ci",
		Short:         "PHPerf pour la CI : échoue uniquement sur les findings nouveaux",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newBaselineCmd(), newRunCmd())
	return root.Execute()
}

// addPipelineFlags — déclare les flags communs aux deux commandes.
func addPipelineFlags(cmd *cobra.Command, profilePath, rulesPath *string) {
	cmd.Flags().StringVar(profilePath, "profile", "", "profil XHProf sérialisé en JSON")
	cmd.Flags().StringVar(rulesPath, "rules", "", "règles YAML (modèle : proto/rules.example.yaml)")
}

// newBaselineCmd — régénère intégralement la baseline depuis les findings
// courants (façon phpstan -b : remplacement, pas de fusion).
func newBaselineCmd() *cobra.Command {
	var profilePath, rulesPath, baselinePath string

	cmd := &cobra.Command{
		Use:   "baseline",
		Short: "Régénère la baseline avec les findings courants du profil",
		RunE: func(cmd *cobra.Command, _ []string) error {
			findings, err := evaluateProfile(profilePath, rulesPath)
			if err != nil {
				return err
			}

			file := baselineFile(findings)
			var buf bytes.Buffer
			if err := baseline.Save(&buf, file); err != nil {
				return err
			}
			if err := os.WriteFile(baselinePath, buf.Bytes(), 0o644); err != nil {
				return fmt.Errorf("baseline : écriture de %s : %w", baselinePath, err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Baseline écrite : %s (%d entrée(s))\n", baselinePath, len(file.Entries))
			return nil
		},
	}

	addPipelineFlags(cmd, &profilePath, &rulesPath)
	cmd.Flags().StringVar(&baselinePath, "baseline", defaultBaselinePath, "fichier de baseline à écrire")
	markRequired(cmd)
	return cmd
}

// newRunCmd — compare les findings courants à la baseline et échoue
// (code 1) uniquement s'il existe des findings nouveaux.
func newRunCmd() *cobra.Command {
	var profilePath, rulesPath, scoringPath, baselinePath string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Échoue si des findings n'apparaissent pas dans la baseline",
		RunE: func(cmd *cobra.Command, _ []string) error {
			findings, err := evaluateProfile(profilePath, rulesPath)
			if err != nil {
				return err
			}

			bl, err := loadBaselineFile(baselinePath)
			if err != nil {
				return err
			}

			res := baseline.Diff(findings, bl)

			priorities, err := scoreForDisplay(findings, scoringPath)
			if err != nil {
				return err
			}
			printRunReport(cmd.OutOrStdout(), res, priorities)

			if len(res.New) > 0 {
				return &codedError{
					code: 1,
					msg:  fmt.Sprintf("%d nouveau(x) finding(s) hors baseline (sur %d au total)", len(res.New), len(findings)),
				}
			}
			return nil
		},
	}

	addPipelineFlags(cmd, &profilePath, &rulesPath)
	cmd.Flags().StringVar(&scoringPath, "scoring", "", "pondérations YAML optionnelles (défauts embarqués sinon)")
	cmd.Flags().StringVar(&baselinePath, "baseline", defaultBaselinePath, "fichier de baseline à comparer")
	markRequired(cmd)
	return cmd
}

// markRequired — profil et règles sont indispensables au pipeline.
func markRequired(cmd *cobra.Command) {
	for _, name := range []string{"profile", "rules"} {
		if err := cmd.MarkFlagRequired(name); err != nil {
			panic(fmt.Sprintf("flag %s : %v", name, err)) // invariant : flag déclaré juste avant
		}
	}
}
