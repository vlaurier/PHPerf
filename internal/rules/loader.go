package rules

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"

	"gopkg.in/yaml.v3"
)

var (
	ruleIDPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

	validSeverities      = []Severity{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow}
	validEfforts         = []Effort{EffortLow, EffortMedium, EffortHigh}
	validControllability = []Controllability{Controllable, PartialControl, NoControl}
)

// Load — décode et valide un fichier de règles YAML. Le décodage est strict :
// tout champ inconnu est rejeté (aligné avec proto/rules.schema.json).
func Load(data []byte) ([]Rule, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	var file struct {
		Rules []Rule `yaml:"rules"`
	}
	if err := dec.Decode(&file); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("rules : fichier vide")
		}
		return nil, fmt.Errorf("rules : %w", err)
	}
	if len(file.Rules) == 0 {
		return nil, errors.New("rules : aucune règle définie")
	}

	seen := make(map[string]bool, len(file.Rules))
	for i := range file.Rules {
		r := &file.Rules[i]
		if err := validateRule(r); err != nil {
			return nil, fmt.Errorf("rules : règle #%d (%q) : %w", i+1, r.ID, err)
		}
		if seen[r.ID] {
			return nil, fmt.Errorf("rules : règle #%d : id dupliqué %q", i+1, r.ID)
		}
		seen[r.ID] = true
	}
	return file.Rules, nil
}

// validateRule — vérifie la cohérence d'une règle : identité, champs requis,
// enums, et présence d'au moins un critère de match exploitable.
func validateRule(r *Rule) error {
	if !ruleIDPattern.MatchString(r.ID) {
		return fmt.Errorf("id %q invalide (attendu : minuscules, chiffres, tirets)", r.ID)
	}
	if r.Name == "" {
		return errors.New("name manquant")
	}
	if r.Description == "" {
		return errors.New("description manquante")
	}
	if err := validateEnum("severity", r.Severity, validSeverities); err != nil {
		return err
	}
	if err := validateEnum("effort", r.Effort, validEfforts); err != nil {
		return err
	}
	if err := validateEnum("controllability", r.Controllability, validControllability); err != nil {
		return err
	}

	m := &r.Match
	if m.FunctionPattern == "" && m.CallCountThreshold == nil && m.MemoryPerCallThresholdMB == nil {
		return errors.New("match : au moins un critère requis")
	}
	if m.FunctionPattern != "" {
		if _, err := regexp.Compile(m.FunctionPattern); err != nil {
			return fmt.Errorf("match.function_pattern %q invalide : %w", m.FunctionPattern, err)
		}
	}
	return nil
}

// validateEnum — vérifie que value figure parmi les valeurs autorisées.
func validateEnum[T ~string](field string, value T, allowed []T) error {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return fmt.Errorf("%s : %q invalide (valeurs possibles : %v)", field, string(value), allowed)
}
