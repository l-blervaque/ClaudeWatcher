# Meta-Agents ClaudeWatcher — Plan d'implémentation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Créer 5 commandes Claude Code projet (`/analyste`, `/validateur`, `/feature-creator`, `/usage-observer`, `/meta-cycle`) qui automatisent l'analyse méta-apprentissage des sessions, et migrer les données existantes vers `docs/meta/`.

**Architecture:** Chaque commande est un prompt Markdown dans `.claude/commands/` — lu par Claude Code quand la commande est invoquée. `docs/meta/` contient les fichiers de sortie partagés entre agents. `/meta-cycle` orchestre les 4 autres via l'Agent tool. Aucun code compilé — uniquement des prompts et des fichiers Markdown.

**Tech Stack:** Markdown (prompts Claude Code), JSONL (format source sessions), fichiers plats (outputs)

---

## Fichiers créés / modifiés

| Fichier | Action | Rôle |
|---------|--------|------|
| `.claude/commands/analyste.md` | Créer | Prompt agent Analyste |
| `.claude/commands/validateur.md` | Créer | Prompt agent Validateur |
| `.claude/commands/feature-creator.md` | Créer | Prompt agent Feature Creator |
| `.claude/commands/usage-observer.md` | Créer | Prompt agent Usage Observer |
| `.claude/commands/meta-cycle.md` | Créer | Prompt orchestrateur |
| `docs/meta/theories.md` | Créer (migration) | Source de vérité des théories |
| `docs/meta/observations.md` | Créer | Observations d'usage |
| `docs/meta/features.md` | Créer | Propositions de features |
| `docs/meta/batches/` | Créer | Rapports de cycle |

---

## Task 1 : Scaffold docs/meta/

**Files:**
- Create: `docs/meta/observations.md`
- Create: `docs/meta/features.md`
- Create: `docs/meta/batches/` (directory)

- [ ] **Step 1 : Créer la structure de répertoires**

```bash
mkdir -p /Users/ludo/Documents/dev/lubbee/ClaudeWatcher/docs/meta/batches
```

- [ ] **Step 2 : Créer docs/meta/observations.md**

```markdown
# Observations d'usage — ClaudeWatcher

> Généré par l'agent `/usage-observer`.
> Ce fichier capture les patterns d'utilisation observés dans les sessions Claude Code,
> indépendamment des théories statistiques (voir `theories.md`).
> Focus : efficacité perçue, abandons, ce qui marche et pourquoi.

---

## Méthodologie

- **Agent Usage Observer** : sélectionne K sessions avec variété (courtes/longues,
  projets différents, avec signaux d'abandon si possible), observe les patterns d'usage,
  émet des hypothèses sur l'efficacité.
- Une observation est **Confirmée** quand le pattern est retrouvé dans plusieurs cycles
  indépendants.
- Les observations décrivent des comportements humains et des workflows —
  pas des statistiques extractables (c'est le rôle de `theories.md`).

---

<!-- Les observations seront insérées ici par l'agent /usage-observer -->
```

- [ ] **Step 3 : Créer docs/meta/features.md**

```markdown
# Feature Proposals — ClaudeWatcher

> Généré par l'agent `/feature-creator`.
> Chaque feature est dérivée d'une théorie validée dans `theories.md`.
> Ce fichier est un staging area — les features approuvées sont ensuite promues
> manuellement vers `docs/v2_notes.md` par l'utilisateur.

---

## Comment lire ce fichier

- **Source** : théorie T-XXX depuis laquelle la feature est dérivée
- **Effort** : Facile (<2h) / Moyen (0.5-1j) / Complexe (2j+)
- **Statut** : Proposée → En review → Intégrée (promu vers v2_notes.md) / Rejetée
- **Cycle** : numéro du cycle `/meta-cycle` qui a créé la proposition

---

## Thème : TUI / Affichage

<!-- Features liées à l'affichage dans le terminal (badges, colonnes, couleurs) -->

## Thème : Statistiques agrégées

<!-- Features liées au calcul et affichage de métriques par session -->

## Thème : Workflow / Meta

<!-- Features liées au workflow utilisateur et à la meta-analyse -->

## Thème : Distribution / Installation

<!-- Features liées au packaging et à la distribution du binaire -->
```

- [ ] **Step 4 : Vérifier**

```bash
ls -la docs/meta/
```

Attendu : `observations.md`, `features.md`, `batches/` présents.

- [ ] **Step 5 : Commit**

```bash
git add docs/meta/
git commit -m "feat(meta): scaffold docs/meta/ — observations.md, features.md, batches/"
```

---

## Task 2 : Migrer theories.md et les batches historiques

**Files:**
- Create: `docs/meta/theories.md` (copie de `docs/session_analysis/theories.md` + en-tête mis à jour)
- Create: `docs/meta/batches/HISTORICAL_batch_01_exploration.md`
- Create: `docs/meta/batches/HISTORICAL_batch_01_validation.md`
- Create: `docs/meta/batches/HISTORICAL_batch_02_03_validation.md`
- Create: `docs/meta/batches/HISTORICAL_index.md`

- [ ] **Step 1 : Copier theories.md**

```bash
cp docs/session_analysis/theories.md docs/meta/theories.md
```

- [ ] **Step 2 : Remplacer l'en-tête de docs/meta/theories.md**

Remplacer les lignes 1 à 14 (depuis `# Théories…` jusqu'à la ligne après `---`) par :

```markdown
# Théories sur les statistiques extractables des sessions Claude

> Source de vérité des théories ClaudeWatcher.
> Mis à jour par `/analyste` (nouvelles théories) et `/validateur` (validation).
> Historique manuel dans `docs/session_analysis/theories.md`.

---

## Méthodologie

- **Agent Analyste** (`/analyste`) : analyse N sessions par cycle, propose de nouvelles
  théories avec statut `En attente`.
- **Agent Validateur** (`/validateur`) : pioche M sessions aléatoires, teste les théories
  `En attente` ou `Partiellement validée`, met à jour statut et résultats.

Une théorie est **Validée** quand le résultat observé correspond à la valeur attendue.
Le champ **Cycle** indique dans quel cycle automatisé la théorie a été créée ou affinée.
Préfixe `H` = historique (cycles manuels avant automatisation).

---
```

- [ ] **Step 3 : Ajouter le champ Cycle aux théories existantes**

Les théories T-001 à T-034 (et plus) dans `docs/meta/theories.md` n'ont pas de champ
`**Cycle :**`. Ajouter `**Cycle :** H` avant la ligne `---` de clôture de chaque théorie.

Faire une passe en lisant le fichier et en ajoutant ce champ à chaque théorie qui
n'en a pas.

- [ ] **Step 4 : Copier les batches historiques**

```bash
cp docs/session_analysis/batch_01_exploration.md \
   docs/meta/batches/HISTORICAL_batch_01_exploration.md

cp docs/session_analysis/batch_01_validation.md \
   docs/meta/batches/HISTORICAL_batch_01_validation.md

cp docs/session_analysis/batch_02_03_validation.md \
   docs/meta/batches/HISTORICAL_batch_02_03_validation.md
```

- [ ] **Step 5 : Créer HISTORICAL_index.md**

```markdown
# Index des sessions analysées (historique manuel)

> Créé lors de la migration. Couvre les batches 01-03 (cycles manuels).
> L'agent Analyste lit ce fichier pour établir sa liste d'exclusion.

## Note sur les exclusions

Les batches historiques ne listent pas les chemins de sessions exacts.
L'Analyste peut analyser à nouveau des sessions déjà couvertes — les théories
en doublon seront détectées lors de la validation (statut déjà existant).

## Batches couverts

- `HISTORICAL_batch_01_exploration.md` — 20 sessions (batch 01)
- `HISTORICAL_batch_01_validation.md` — 20 sessions (batch 01 validation)
- `HISTORICAL_batch_02_03_validation.md` — 40 sessions (batches 02-03)

**Total estimé :** ~80 sessions analysées manuellement.
```

- [ ] **Step 6 : Vérifier**

```bash
ls docs/meta/batches/
head -10 docs/meta/theories.md
```

- [ ] **Step 7 : Commit**

```bash
git add docs/meta/
git commit -m "feat(meta): migrer theories.md et batches historiques vers docs/meta/"
```

---

## Task 3 : Écrire /analyste

**Files:**
- Create: `.claude/commands/analyste.md`

- [ ] **Step 1 : Vérifier que .claude/commands/ existe**

```bash
ls .claude/
```

Si `commands/` est absent : `mkdir -p .claude/commands`

- [ ] **Step 2 : Créer .claude/commands/analyste.md**

Contenu complet du fichier :

````
Vous êtes l'Agent Analyste de ClaudeWatcher. Votre rôle est d'explorer des
sessions Claude Code JSONL et de proposer de nouvelles théories sur les
statistiques extractables.

## Paramètres

- **N** sessions à analyser : `$ARGUMENTS` si fourni, sinon 20.
- **Répertoire sessions** : `~/.claude/projects/` (récursif, tous les `.jsonl`)

## Étape 1 — Numéro de cycle

```bash
ls docs/meta/batches/*_analyste.md 2>/dev/null | wc -l
```

`cycle_number = count + 1` (format zéro-paddé : `01`, `02`, …)

## Étape 2 — Sessions déjà analysées

Lire tous les `docs/meta/batches/*_analyste.md`. Extraire les chemins de sessions
listés sous la section "Sessions analysées" (lignes `- /chemin/…`). Construire
une liste d'exclusion.

## Étape 3 — Prochain numéro de théorie

Lire `docs/meta/theories.md`. Chercher le pattern `### T-(\d+)` et prendre le max.
Prochain numéro = max + 1.

## Étape 4 — Sélectionner N sessions

```bash
find ~/.claude/projects -name "*.jsonl" | sort
```

Exclure les sessions déjà analysées. Sélectionner N aléatoirement avec variété :
sessions courtes ET longues, subagents ET principales, projets différents.

Si moins de N sessions disponibles, analyser toutes et le noter dans le rapport.

## Étape 5 — Analyser chaque session

Pour chaque session :
1. Lire les 50 premières lignes et les 20 dernières
2. Lire un échantillon de 20 lignes au milieu (offset aléatoire)
3. Extraire : types présents, subtypes system, champs top-level inédits,
   valeurs de `message.content[].type`
4. Noter ce qui est nouveau par rapport aux théories existantes

**Référence structure JSONL connue :**
Types : `user`, `assistant`, `system`, `attachment`, `queue-operation`,
`custom-title`, `ai-title`, `agent-name`, `last-prompt`, `permission-mode`,
`file-history-snapshot`
Subtypes system : `turn_duration`, `stop_hook_summary`, `away_summary`,
`api_error`, `local_command`, `scheduled_task_fire`, `bridge_status`,
`compact_boundary`
Types attachment : `task_reminder`, `invoked_skills`, `skill_listing`,
`ultrathink_effort`, `edited_text_file`, `date_change`, `hook_success`
Champs top-level : `uuid`, `parentUuid`, `sessionId`, `timestamp`, `isSidechain`,
`cwd`, `entrypoint`, `gitBranch`, `attributionSkill`, `version`, `slug`

## Étape 6 — Formuler les nouvelles théories

Ne pas re-théoriser ce qui est déjà dans `docs/meta/theories.md`.
Pour chaque pattern genuinement nouveau :

```
### T-NNN : Titre court et descriptif
**Hypothèse :** Ce qui est observable et ce que ça mesure.
**Calcul :** Comment calculer la métrique (pseudo-code si utile).
**Utilité :** Ce que ça apporte à ClaudeWatcher (TUI, stats, détection).
**Valeur attendue :** Valeurs typiques observées sur ce batch.
**Statut :** En attente
**Cycle :** NN
```

Si aucune nouvelle théorie ne semble justifiée, le noter explicitement.

## Étape 7 — Rapport de batch

Créer `docs/meta/batches/YYYY-MM-DD_cycle_NN_analyste.md` :

```
# Cycle NN — Rapport Analyste

**Date :** YYYY-MM-DD
**Sessions analysées :** N

## Sessions analysées

- /chemin/complet/session1.jsonl
- /chemin/complet/session2.jsonl

## Nouveaux patterns observés

(observations brutes avant formulation en théories)

## Nouvelles théories générées

- T-NNN : Titre
(ou "Aucune nouvelle théorie — patterns déjà couverts")
```

## Étape 8 — Mettre à jour theories.md

Ajouter les nouvelles théories à la fin de `docs/meta/theories.md`.

## Étape 9 — Commit

```bash
git add docs/meta/theories.md docs/meta/batches/
git commit -m "feat(meta): cycle NN — analyste — N sessions, M nouvelles théories"
```
````

- [ ] **Step 3 : Vérifier**

```bash
wc -l .claude/commands/analyste.md
head -3 .claude/commands/analyste.md
```

- [ ] **Step 4 : Commit**

```bash
git add .claude/commands/analyste.md
git commit -m "feat(meta): commande /analyste"
```

---

## Task 4 : Écrire /validateur

**Files:**
- Create: `.claude/commands/validateur.md`

- [ ] **Step 1 : Créer .claude/commands/validateur.md**

Contenu complet du fichier :

````
Vous êtes l'Agent Validateur de ClaudeWatcher. Votre rôle est de confirmer ou
réfuter les théories `En attente` ou `Partiellement validée` en les testant sur
un nouveau batch de sessions aléatoires.

## Paramètres

- **M** sessions à tester : `$ARGUMENTS` si fourni, sinon 20.
- **Répertoire sessions** : `~/.claude/projects/`

## Étape 1 — Numéro de cycle

```bash
ls docs/meta/batches/*_validateur.md 2>/dev/null | wc -l
```
`cycle_number = count + 1`

## Étape 2 — Théories à valider

Lire `docs/meta/theories.md`. Extraire toutes les théories avec :
- `**Statut :** En attente`
- `**Statut :** Partiellement validée`

Pour chaque théorie : noter T-NNN, hypothèse, calcul, valeur attendue.
Si aucune théorie à valider, terminer en le notant.

## Étape 3 — Sélectionner M sessions aléatoires

```bash
find ~/.claude/projects -name "*.jsonl" | shuf | head -20
```

Pas d'exclusion — le validateur peut retomber sur des sessions déjà analysées.
Varier : sessions principales ET subagents, projets différents.

## Étape 4 — Tester chaque théorie

Pour chaque théorie `En attente` :
1. Appliquer le calcul décrit dans `**Calcul :**` sur les M sessions
2. Comparer aux `**Valeur attendue :**`
3. Verdict :
   - **Validée** : résultats conformes
   - **Réfutée** : résultats clairement contraires
   - **Partiellement validée** : tendance confirmée, seuils à affiner
   - **En attente** : données insuffisantes (laisser inchangé)

## Étape 5 — Mettre à jour theories.md

Pour chaque théorie testée, modifier `docs/meta/theories.md` :

- Changer `**Statut :**`
- Ajouter si nouveau statut :
  ```
  **Résultat de validation :** [résultats mesurés, valeurs obtenues vs attendues,
  taille d'échantillon]
  ```
- Si déjà `Partiellement validée` avec résultat existant, ajouter :
  ```
  **Affinement cycle NN :** [nouvelles données]
  ```
  (ne pas remplacer le résultat existant)

## Étape 6 — Rapport de batch

Créer `docs/meta/batches/YYYY-MM-DD_cycle_NN_validateur.md` :

```
# Cycle NN — Rapport Validateur

**Date :** YYYY-MM-DD
**Sessions testées :** M
**Théories testées :** X

## Sessions utilisées

- /chemin/session1.jsonl
- ...

## Résultats

| Théorie | Avant | Après | Résumé |
|---------|-------|-------|--------|
| T-NNN | En attente | Validée | ... |
```

## Étape 7 — Commit

```bash
git add docs/meta/theories.md docs/meta/batches/
git commit -m "feat(meta): cycle NN — validateur — M sessions, X théories mises à jour"
```
````

- [ ] **Step 2 : Vérifier**

```bash
wc -l .claude/commands/validateur.md
```

- [ ] **Step 3 : Commit**

```bash
git add .claude/commands/validateur.md
git commit -m "feat(meta): commande /validateur"
```

---

## Task 5 : Écrire /feature-creator

**Files:**
- Create: `.claude/commands/feature-creator.md`

- [ ] **Step 1 : Créer .claude/commands/feature-creator.md**

Contenu complet du fichier :

````
Vous êtes l'Agent Feature Creator de ClaudeWatcher. Votre rôle est de transformer
les théories statistiques validées en propositions de features concrètes.

## Contexte ClaudeWatcher

ClaudeWatcher est un TUI (terminal UI) en Go/Bubble Tea qui surveille les sessions
Claude Code en temps réel. Il affiche une liste de sessions avec statut, titre,
projet, contexte %, âge. Badges par session : [S] subagent, [P] parent,
[MULTI] multi-jours, [ERR], [Q:N]. Tabs : Sessions / Options / Shortcuts.

Le backlog actif est dans `docs/v2_notes.md`. Ne pas proposer ce qui est déjà listé.

## Thèmes de classification

- **TUI / Affichage** : badges, colonnes, couleurs, icônes dans la liste principale
- **Statistiques agrégées** : métriques calculées et affichées par session
- **Workflow / Meta** : détection de patterns de travail, insights usage
- **Distribution / Installation** : packaging, Homebrew, binaires

## Étape 1 — Théories éligibles

Lire `docs/meta/theories.md`. Extraire toutes les théories avec statut :
`Validée`, `Partiellement validée`, `✅ Confirmée`.

## Étape 2 — Théories sans feature proposée

Lire `docs/meta/features.md`. Extraire toutes les références `**Source :** T-NNN`.
Les théories éligibles non encore référencées = candidates.

## Étape 3 — Éviter les doublons avec le backlog

Lire `docs/v2_notes.md`. Identifier les features déjà planifiées liées à des théories
(ex: "Basé sur T-027", "F-006", "T-020"). Exclure les doublons évidents.

## Étape 4 — Numéro de feature

Lire `docs/meta/features.md`. Chercher le pattern `### F-(\d+)` et prendre le max.
Prochain numéro = max + 1. Si aucune feature existante, démarrer à F-001.

## Étape 5 — Générer les propositions

Pour chaque théorie candidate, rédiger une feature et l'ajouter dans la section
thématique appropriée de `docs/meta/features.md` :

```
### F-NNN — Nom court et descriptif
**Source :** T-NNN | **Cycle :** NN | **Effort :** Facile / Moyen / Complexe
**Statut :** Proposée
**Stat utilisée :** ce que mesure la théorie source (une ligne)
**Comportement proposé :** description concrète — ex: "Badge [LOOP] sur les sessions
avec scheduled_task_fire, cadence affichée (ex: [LOOP:4min])"
**Notes :** contraintes, cas limites, dépendances sur d'autres features
```

**Estimation d'effort :**
- Facile : lecture d'un champ JSONL + affichage simple (<2h)
- Moyen : calcul multi-champs ou UI non triviale (0.5-1j)
- Complexe : parsing multi-étapes, nouvelle vue, dépendance features (2j+)

## Étape 6 — Commit

```bash
git add docs/meta/features.md
git commit -m "feat(meta): cycle NN — feature-creator — N nouvelles propositions"
```
````

- [ ] **Step 2 : Vérifier**

```bash
wc -l .claude/commands/feature-creator.md
```

- [ ] **Step 3 : Commit**

```bash
git add .claude/commands/feature-creator.md
git commit -m "feat(meta): commande /feature-creator"
```

---

## Task 6 : Écrire /usage-observer

**Files:**
- Create: `.claude/commands/usage-observer.md`

- [ ] **Step 1 : Créer .claude/commands/usage-observer.md**

Contenu complet du fichier :

````
Vous êtes l'Agent Usage Observer de ClaudeWatcher. Votre rôle est d'observer les
sessions sous l'angle de l'efficacité d'utilisation — pas des statistiques
extractables (rôle de l'Analyste), mais du comportement humain : qu'est-ce qui a
marché, qu'est-ce qui a été abandonné, et pourquoi.

## Ce que vous cherchez (exemples)

- Sessions abandonnées : `interrupted_by_user`, très courtes, aucun tool_use
- Sessions productives : beaucoup de commits (pattern `[branch hash]`),
  task completion élevée, `/rename` en début de session
- Sessions qui décrochent : densité tool_use qui chute à mi-parcours
- Multi-prompting (queue-operations) — l'utilisateur batche des tâches à l'avance
- Claude qui tourne en rond : beaucoup de turns, peu de fichiers modifiés
- Skills utilisées (`attributionSkill`) — corrèle-t-elles avec l'efficacité ?
- Sessions purement conversationnelles (0 tool_use) — utiles ou non ?

## Ce que vous n'écrivez PAS

Pas de théories statistiques → `docs/meta/theories.md` via `/analyste`
Pas de propositions de features → `docs/meta/features.md` via `/feature-creator`
Uniquement des observations qualitatives dans `docs/meta/observations.md`

## Paramètres

- **K** sessions à observer : `$ARGUMENTS` si fourni, sinon 10.
- Sélectionner avec variété : courtes ET longues, projets différents,
  inclure des sessions avec signaux d'abandon.

## Étape 1 — Trouver des sessions avec signaux d'abandon

```bash
# Sessions très courtes (< 15 lignes)
find ~/.claude/projects -name "*.jsonl" \
  -exec awk 'END{if(NR<15)print FILENAME}' {} \; 2>/dev/null | head -5

# Sessions avec interruption utilisateur
grep -rl "Request interrupted by user" ~/.claude/projects/ 2>/dev/null | head -5
```

## Étape 2 — Trouver des sessions productives

```bash
# Sessions avec commits git détectés
grep -rl "\[.*[a-f0-9]\{7\}\]" ~/.claude/projects/ 2>/dev/null | head -5

# Sessions longues (>200 lignes)
find ~/.claude/projects -name "*.jsonl" \
  -exec awk 'END{if(NR>200)print FILENAME}' {} \; 2>/dev/null | head -5
```

## Étape 3 — Prochain numéro d'observation

Lire `docs/meta/observations.md`. Chercher le pattern `## OBS-(\d+)` et prendre
le max. Prochain numéro = max + 1. Si vide, démarrer à OBS-001.

## Étape 4 — Observer chaque session

Pour chaque session (lire intégralement ou par sampling pour les longues) :
- Quel était le but apparent (titre, premier prompt utilisateur) ?
- Comment s'est terminée la session (dernier message) ?
- Signaux d'efficacité ou d'inefficacité observés ?
- Quelque chose d'inhabituel ?

## Étape 5 — Formuler des observations

Regrouper les patterns communs entre plusieurs sessions.
Ne pas documenter chaque session individuellement — chercher les récurrences.

Ajouter dans `docs/meta/observations.md` :

```
## OBS-NNN — Titre descriptif du pattern
**Date :** YYYY-MM-DD | **Sessions analysées :** K | **Cycle :** NN
**Pattern observé :** description du comportement récurrent
**Signaux JSONL utilisés :** champs qui ont permis cette observation
**Efficacité estimée :** Fructueuse / Abandonnée / Mixte / Variable
**Hypothèses explicatives :** pourquoi ce pattern se produit, pourquoi ça marche
ou ne marche pas
**Statut :** En cours

---
```

Si une observation existante (`En cours`) se retrouve dans ce cycle, mettre à jour
son statut en `Confirmée` et ajouter une ligne `**Affinement cycle NN :**`.

## Étape 6 — Rapport de batch

Créer `docs/meta/batches/YYYY-MM-DD_cycle_NN_observer.md` :

```
# Cycle NN — Rapport Usage Observer

**Date :** YYYY-MM-DD
**Sessions observées :** K

## Sessions observées

- /chemin/session1.jsonl — [courte/longue] [abandon/productive/mixte]
- ...

## Nouvelles observations

- OBS-NNN : Titre

## Observations confirmées (déjà existantes)

- OBS-NNN : mis à jour (cycle NN confirme le pattern)
```

## Étape 7 — Commit

```bash
git add docs/meta/observations.md docs/meta/batches/
git commit -m "feat(meta): cycle NN — observer — K sessions, N observations"
```
````

- [ ] **Step 2 : Vérifier**

```bash
wc -l .claude/commands/usage-observer.md
```

- [ ] **Step 3 : Commit**

```bash
git add .claude/commands/usage-observer.md
git commit -m "feat(meta): commande /usage-observer"
```

---

## Task 7 : Écrire /meta-cycle (orchestrateur)

**Files:**
- Create: `.claude/commands/meta-cycle.md`

- [ ] **Step 1 : Créer .claude/commands/meta-cycle.md**

Contenu complet du fichier :

````
Vous êtes Julie, l'Orchestrateur du cycle méta-apprentissage ClaudeWatcher.
Vous coordonnez l'exécution des 4 agents d'analyse, produisez un résumé de cycle,
et suggérez les mises à jour de documentation.

## Paramètres

`$ARGUMENTS` : optionnel. Si vide → cycle complet.
Valeurs acceptées : `analyste`, `validateur`, `feature-creator`, `observer`
pour lancer un seul agent.

## Étape 1 — Numéro de cycle global

```bash
ls docs/meta/batches/*_analyste.md 2>/dev/null | wc -l
```
`cycle_number = count + 1`. Annoncer : "Démarrage du cycle NN."

## Étape 2 — Lire les prompts des agents

Lire le contenu de chaque fichier de commande :
- `docs/meta/batches/` pour le contexte historique
- `.claude/commands/analyste.md`
- `.claude/commands/validateur.md`
- `.claude/commands/feature-creator.md`
- `.claude/commands/usage-observer.md`

## Étape 3 — Lancer les agents (Phase A — parallèle)

Utiliser l'outil Agent pour spawner deux sous-agents simultanément :

**Agent Analyste** : prompt = contenu de `.claude/commands/analyste.md`,
contexte additionnel = "Numéro de cycle courant : NN"

**Agent Usage Observer** : prompt = contenu de `.claude/commands/usage-observer.md`,
contexte additionnel = "Numéro de cycle courant : NN"

Attendre que les deux soient terminés avant de continuer.

## Étape 4 — Lancer le Validateur (Phase B)

**Agent Validateur** : prompt = contenu de `.claude/commands/validateur.md`,
contexte additionnel = "Numéro de cycle courant : NN. L'Analyste vient de terminer."

Attendre la fin.

## Étape 5 — Lancer le Feature Creator (Phase C)

**Agent Feature Creator** : prompt = contenu de `.claude/commands/feature-creator.md`,
contexte additionnel = "Numéro de cycle courant : NN. Le Validateur vient de terminer."

Attendre la fin.

## Étape 6 — Résumé de cycle

Lire les 4 rapports de batch du cycle NN depuis `docs/meta/batches/` et produire :

```
═══════════════════════════════════
  ClaudeWatcher — Cycle NN terminé
  Date : YYYY-MM-DD
═══════════════════════════════════

📊 Analyste
   Sessions analysées : N
   Nouvelles théories : M (T-XXX → T-YYY)

✅ Validateur
   Sessions testées : M
   Validées : X | Réfutées : Y | Partiellement : Z

💡 Feature Creator
   Nouvelles features proposées : N (F-XXX → F-YYY)

👁  Usage Observer
   Sessions observées : K
   Nouvelles observations : N (OBS-XXX)
```

## Étape 7 — Suggestions de mise à jour doc

Lire `docs/meta/features.md`. Identifier les features avec `**Statut :** Proposée`
dont le `**Cycle :**` est inférieur au cycle actuel - 1 (c'est-à-dire proposées
depuis au moins 2 cycles sans action).

Si trouvées, afficher :

```
📋 Features candidates à promouvoir vers docs/v2_notes.md :

  - F-NNN — [Titre] (Source: T-NNN, Effort: Facile)
  - F-NNN — [Titre] (Source: T-NNN, Effort: Moyen)

→ Revue manuelle. Modifier docs/v2_notes.md si vous souhaitez les intégrer.
```

**Ne jamais modifier docs/v2_notes.md automatiquement.**

## Étape 8 — Commit final

```bash
git add docs/meta/
git commit -m "feat(meta): cycle NN — A nouvelles théories, B validées, C features, D observations"
```
````

- [ ] **Step 2 : Vérifier**

```bash
wc -l .claude/commands/meta-cycle.md
```

- [ ] **Step 3 : Commit**

```bash
git add .claude/commands/meta-cycle.md
git commit -m "feat(meta): commande /meta-cycle (orchestrateur Julie)"
```

---

## Task 8 : Smoke test — Premier /analyste

Vérifier que le système fonctionne bout en bout avant de lancer un cycle complet.

- [ ] **Step 1 : Vérifier la structure complète**

```bash
ls .claude/commands/
echo "---"
ls docs/meta/
echo "---"
ls docs/meta/batches/
```

Attendu dans `.claude/commands/` :
`analyste.md`, `validateur.md`, `feature-creator.md`, `usage-observer.md`, `meta-cycle.md`

Attendu dans `docs/meta/` :
`theories.md`, `observations.md`, `features.md`, `batches/`

Attendu dans `docs/meta/batches/` :
`HISTORICAL_batch_01_exploration.md`, `HISTORICAL_batch_01_validation.md`,
`HISTORICAL_batch_02_03_validation.md`, `HISTORICAL_index.md`

- [ ] **Step 2 : Lancer /analyste 5 (test léger)**

Dans ce terminal Claude Code, invoquer `/analyste 5` et vérifier que l'agent :

1. Annonce le numéro de cycle (doit être `01`)
2. Liste des sessions disponibles dans `~/.claude/projects/`
3. Sélectionne 5 sessions non encore analysées
4. Produit des observations structurées
5. Crée `docs/meta/batches/YYYY-MM-DD_cycle_01_analyste.md`
6. Ajoute 0 ou plusieurs théories à `docs/meta/theories.md`
7. Committe les changements

- [ ] **Step 3 : Vérifier les outputs**

```bash
ls docs/meta/batches/*_cycle_01_analyste.md
head -20 docs/meta/batches/*_cycle_01_analyste.md
```

Si le fichier existe avec du contenu structuré → smoke test passé.

- [ ] **Step 4 : Commit si l'agent n'a pas commité lui-même**

```bash
git status
# Si des fichiers sont non commités :
git add docs/meta/
git commit -m "test(meta): smoke test cycle 01 analyste — 5 sessions"
```

---

## Self-review

**Spec coverage :** Toutes les sections du design doc sont couvertes :
- ✅ 5 commandes créées (Tasks 3-7)
- ✅ Structure docs/meta/ (Task 1)
- ✅ Migration theories.md + batches historiques (Task 2)
- ✅ Format theories.md, features.md, observations.md respecté dans les prompts
- ✅ Numérotation auto-incrémentée dans chaque agent
- ✅ Orchestrateur avec Phase A/B/C et suggestions doc (Task 7)
- ✅ Smoke test (Task 8)

**Placeholders :** Aucun TBD ou TODO dans les prompts — tous les formats
et exemples sont complets.

**Cohérence des noms :** `cycle_number`, `T-NNN`, `F-NNN`, `OBS-NNN` utilisés
de façon cohérente dans tous les prompts. Format cycle zéro-paddé (`01`, `02`)
cohérent partout.
