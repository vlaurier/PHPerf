# AGENTS.md — proto/

> Instructions spécifiques au dossier `proto/` — à lire **en plus** de la racine.

## Contenu du dossier

Schémas, exemples et définitions de formats :

- `rules.example.yaml` — exemple de fichier de règles exploitable par le
  moteur de règles (`internal/rules`).
- `rules.schema.json` — schéma JSON décrivant la structure valide d’un
  fichier de règles YAML.

## Ce qu’il ne faut PAS faire ici

- Aucune logique métier : le format YAML est **déclaratif seulement**.
- Pas de code Go qui exécute des règles : c’est le rôle de `internal/rules/`.
