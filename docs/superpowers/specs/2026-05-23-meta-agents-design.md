# Design — Équipe d'agents meta-apprentissage ClaudeWatcher

**Date :** 2026-05-23
**Statut :** Approuvé

---

## Objectif

Automatiser l'analyse des sessions Claude Code pour en extraire des statistiques,
valider des théories, proposer des features, et observer les patterns d'usage.
Le but est le **méta-apprentissage** : comprendre comment Claude est utilisé, ce
qui marche, ce qui ne marche pas, et en dériver des améliorations concrètes pour
ClaudeWatcher.

---

## Architecture : 5 commandes projet

Les agents sont des fichiers Markdown dans `.claude/commands/` — invocables
directement comme slash commands Claude Code (`/analyste`, `/meta-cycle`, etc.).

| Commande | Rôle | Lit | Écrit |
|----------|------|-----|-------|
| `/analyste` | Explore N sessions JSONL, propose de nouvelles théories | Sessions `~/.claude/projects/` | `docs/meta/theories.md` + rapport batch |
| `/validateur` | Pioche M sessions aléatoires, confirme/réfute les théories "En attente" | `docs/meta/theories.md` + sessions | `docs/meta/theories.md` (statuts) + rapport batch |
| `/feature-creator` | Pour chaque théorie Validée sans feature associée, génère une proposition | `docs/meta/theories.md` | `docs/meta/features.md` |
| `/usage-observer` | Analyse des sessions sous l'angle méta-usage (efficacité, abandons, patterns) | Sessions `~/.claude/projects/` | `docs/meta/observations.md` + rapport batch |
| `/meta-cycle` | Orchestrateur — coordonne les 4 agents, met à jour la doc si applicable | Tous les fichiers meta | Suggestions vers `v2_notes.md` |

---

## Flux d'exécution

```
/meta-cycle
    │
    ├─► /analyste          (nouvelles théories)
    │       │
    │       ▼
    ├─► /validateur        (confirme/réfute les théories En attente)
    │       │
    │       ▼
    ├─► /feature-creator   (propose features depuis théories Validées)
    │
    └─► /usage-observer    (parallèle — indépendant des théories)

    Après les 4 agents :
    └─► Orchestrateur — revue doc (suggère promotions vers v2_notes.md)
```

Analyste et Usage Observer peuvent tourner en parallèle.
Validateur attend l'Analyste. Feature Creator attend le Validateur.

---

## Structure de fichiers

```
docs/meta/
  theories.md         — source de vérité des théories (migration de session_analysis/)
  observations.md     — observations d'usage (Usage Observer uniquement)
  features.md         — propositions de features classées par thème
  batches/
    YYYY-MM-DD_cycle_NN_analyste.md
    YYYY-MM-DD_cycle_NN_validateur.md
    YYYY-MM-DD_cycle_NN_observer.md

.claude/commands/
  analyste.md
  validateur.md
  feature-creator.md
  usage-observer.md
  meta-cycle.md
```

`docs/session_analysis/` reste en place pour l'historique — les nouveaux cycles
écrivent dans `docs/meta/`.

---

## Format des outputs

### `theories.md`

Même format que l'existant (`session_analysis/theories.md`), enrichi d'un champ
`Cycle` indiquant quel batch a produit/validé la théorie.

```markdown
### T-XXX : Titre
**Hypothèse :** ...
**Calcul :** ...
**Utilité :** ...
**Valeur attendue :** ...
**Statut :** En attente / Validée / Réfutée / Partiellement validée
**Résultat de validation :** ...
**Cycle :** NN
```

### `features.md`

Classé par thème, lié à la théorie source. Statut suit le cycle de vie feature.

```markdown
## Thème : TUI / Affichage

### F-XXX — Nom de la feature
**Source :** T-XXX | **Effort :** Facile / Moyen / Complexe
**Statut :** Proposée / En review / Intégrée
**Stat utilisée :** description courte
**Comportement proposé :** ...

## Thème : Statistiques agrégées
## Thème : Distribution / Installation
## Thème : Workflow / Meta
```

### `observations.md`

Numérotées, indépendantes des théories. Focus sur l'usage humain de Claude.

```markdown
## OBS-001 — Titre
**Date :** YYYY-MM-DD | **Sessions analysées :** N | **Cycle :** NN
**Pattern observé :** ...
**Efficacité estimée :** Fructueuse / Abandonnée / Mixte
**Signaux JSONL :** quels champs ont permis cette observation
**Hypothèses explicatives :** pourquoi ça marche / ne marche pas
**Statut :** En cours / Confirmée / Réfutée
```

---

## Comportement de chaque agent

### `/analyste`
- Paramètre : `N` sessions à analyser (défaut : 20), piochées aléatoirement
  parmi les sessions non encore explorées dans ce cycle
- Exclut les sessions déjà couvertes : lit les rapports existants dans `docs/meta/batches/`
  pour extraire les chemins de sessions déjà analysés, puis pioche parmi le reste
- Génère un rapport de batch dans `docs/meta/batches/`
- Ajoute les nouvelles théories dans `theories.md` avec statut `En attente`
- Ne modifie pas les théories existantes

### `/validateur`
- Paramètre : `M` sessions aléatoires (défaut : 20), sans contrainte de nouveauté
- Prend toutes les théories avec statut `En attente` ou `Partiellement validée`
- Met à jour le statut et ajoute un `Résultat de validation` dans `theories.md`
- Génère un rapport de batch dans `docs/meta/batches/`

### `/feature-creator`
- Filtre `theories.md` : théories `Validée` ou `Partiellement validée` sans
  feature déjà proposée dans `features.md`
- Pour chaque stat éligible, génère une proposition de feature
- Classe par thème, estime l'effort, lie à la théorie source
- N'écrase pas les features existantes — append uniquement

### `/usage-observer`
- Paramètre : `K` sessions (défaut : 10), variées (courtes, longues, abandonnées)
- Se concentre sur : intentions détectables, signaux d'abandon, patterns efficaces
- Cherche des patterns récurrents sur plusieurs sessions
- Émet des hypothèses sur les causes d'efficacité/d'échec
- Écrit dans `observations.md` — **jamais dans `theories.md`**

### `/meta-cycle`
- Lance `/analyste`, `/validateur`, `/feature-creator`, `/usage-observer`
  via des sous-agents (Agent tool)
- Paramètres transmis : numéro de cycle `NN` (auto-incrémenté depuis les batches existants)
- En fin de cycle, analyse `features.md` : si des features `Proposée` existent depuis
  >1 cycle, liste les candidatures à promouvoir vers `v2_notes.md`
- **Ne modifie pas `v2_notes.md` automatiquement** — propose une liste, l'utilisateur décide
- Peut être invoqué via `/schedule` pour un cycle automatique hebdomadaire

---

## Scheduling

```bash
# Lancer un cycle maintenant
/meta-cycle

# Programmer un cycle hebdomadaire (via la skill /schedule)
/schedule weekly /meta-cycle
```

Le cycle s'auto-numérote en lisant le dernier batch dans `docs/meta/batches/`.

---

## Migration

Le contenu de `docs/session_analysis/theories.md` est migré dans `docs/meta/theories.md`
au premier cycle. Les batches `batch_01_*`, `batch_02_03_*` sont copiés dans
`docs/meta/batches/` pour la continuité historique.
