# Design : Menu Options + Notifications Sonores

**Date :** 2026-05-21  
**Statut :** Approuvé

---

## Objectif

Ajouter à ClaudeWatcher un panneau Options (touche `o`) permettant d'activer des notifications sonores lors des transitions de statut de session. La configuration est persistée entre les sessions.

---

## Architecture

### Nouveaux composants

**`internal/config/config.go`**  
Struct `Config` avec chargement/sauvegarde JSON vers `~/.config/claudewatcher/config.json`.

```go
type Config struct {
    SoundEnabled bool   `json:"sound_enabled"`
    SoundName    string `json:"sound_name"` // "glass", "ping", "funk"
}
```

- `Load()` : lit le fichier, retourne les valeurs par défaut si absent (`sound_enabled: false, sound_name: "glass"`)
- `Save(cfg Config)` : écrit le fichier, crée le répertoire si nécessaire

**`internal/audio/audio.go`**  
Fonction `Play(name string)` qui lance `afplay /System/Library/Sounds/<Name>.aiff` via `os/exec` dans une goroutine. Non-bloquant. Silencieux en cas d'erreur.

**`model.go`** — champs ajoutés au `Model` :

```go
options    bool
optCursor  int
cfg        config.Config
prevStatus map[string]session.Status
```

---

## Panneau Options

Déclenché par la touche `o` depuis la vue liste (pas depuis le détail).  
Rendu par `renderOptions()`, même style visuel que `renderDetail()`.

```
Options

  Sons
  ▌ [ ] Activé
    Son : Glass  Ping  Funk

  esc fermer · j/k nav · espace/enter toggle
```

### Navigation dans le panneau

| Touche | Action |
|--------|--------|
| `j` / `k` | Navigue entre les deux lignes (Activé / Choix du son) |
| `espace` ou `enter` sur "Activé" | Toggle son on/off |
| `enter` sur "Son" | Cycle Glass → Ping → Funk → Glass |
| `esc` | Ferme le panneau et sauvegarde la config |

- Si le son est désactivé, la ligne "Son" est rendue en `dimStyle`
- Le curseur (`▌`) suit `optCursor` (0 = ligne Activé, 1 = ligne Son)

---

## Détection des transitions de statut

À chaque `sessionsMsg` (intervalle de 2s), le model compare les statuts reçus avec `prevStatus` :

- Transition vers `StatusWaiting` → joue le son configuré
- Transition vers `StatusIdle` ou `StatusEnded` → joue le son configuré
- Première apparition d'une session (absente de `prevStatus`) → pas de son
- Après comparaison, `prevStatus` est mis à jour avec les statuts courants

Le son n'est joué que si `cfg.SoundEnabled == true`.

---

## Sons disponibles

| Nom UI | Fichier système |
|--------|----------------|
| Glass  | `/System/Library/Sounds/Glass.aiff` |
| Ping   | `/System/Library/Sounds/Ping.aiff` |
| Funk   | `/System/Library/Sounds/Funk.aiff` |

---

## Fichier de configuration

Chemin : `~/.config/claudewatcher/config.json`

Exemple :
```json
{
  "sound_enabled": true,
  "sound_name": "glass"
}
```

Valeurs par défaut si le fichier est absent : `sound_enabled: false`, `sound_name: "glass"`.

---

## Raccourcis mis à jour

La ligne d'aide du bas devient :

```
j/k nav · enter detail · o options · a all/open · r refresh · q quit
```

---

## Hors périmètre

- Notifications visuelles (hors scope)
- Support Linux/Windows (afplay est macOS uniquement — silencieux ailleurs)
- Volume configurable
