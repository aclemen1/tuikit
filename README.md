> **Ce dépôt est archivé.** Depuis le 12.10.2026, le code vit dans le monorepo privé `aclemen1/jarvis`, sous `tuikit/`. Ce dépôt ne reçoit plus de changements.

# tuikit

Composants de saisie communs aux TUI (office, artefact, oj, due, task, mnemo,
alarm), sur charm.land/bubbletea/v2 v2.0.10, bubbles/v2 v2.2.1 et lipgloss/v2
v2.0.6.

```
go get github.com/aclemen1/tuikit@latest
```

## Convention

- Toute saisie s'ouvre dans une modale centrée sur la vue ; la modale prend
  toutes les touches tant qu'elle est ouverte. Seul le filtre `/` reste en ligne.
- ctrl+s valide (ctrl+enter aussi si le terminal le transmet) ; esc et ctrl+c
  annulent, après une question Oui/Non si la saisie a changé.
- Formulaire : tous les champs visibles ; tab, shift+tab, ↑↓ changent de champ ;
  entrée passe au suivant (insère une ligne dans un champ multiligne) ; une
  erreur s'affiche sous son champ sans bloquer la frappe.

## Intégration

```go
type model struct {
	modal *tuikit.Modal
	width, height int
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tuikit.DoneMsg: // msg.ID, msg.Values
		return m, m.apply(msg)
	case tuikit.CancelMsg:
		return m, nil
	}
	if m.modal.Open() {
		return m, m.modal.Update(msg) // aucun raccourci de la TUI pendant la saisie
	}
	// ... raccourcis de la TUI ; pour ouvrir une saisie :
	// m.modal = tuikit.NewModal("Nouvelle échéance", form).SetSize(m.width, m.height)
}

func (m model) View() tea.View {
	return tea.NewView(tuikit.Overlay(m.vue(), m.modal, m.width, m.height))
}
```

## Contenus

| Constructeur | Saisie | Résultat |
|---|---|---|
| `NewForm(id, champs...)` | formulaire | `DoneMsg.Values` |
| `NewEditor(id, libellé, texte)` | texte libre plein cadre | `Values.String(id)` |
| `NewConfirm(id, question)` | Oui/Non (flèches ou o/n, entrée), Non d'abord ; `.Default(true)` | `DoneMsg` = Oui, `CancelMsg` = Non |
| `NewConfirmTyped(id, question, attendu)` | id à recopier | `DoneMsg` si recopié |
| `NewList(id, items)` | liste verticale : ↑↓ ou j/k, `/` filtre, entrée choisit ; `.Default(valeur)` | `Values.String(id)` = `Item.Value` |

Un contenu écrit hors de tuikit implémente `Content` et rend sa réponse par
`var v tuikit.Values; v.Set(id, valeur)`.

## Champs

| Constructeur | Saisie | Lecture dans `Values` |
|---|---|---|
| `Text(id, libellé)` | une ligne | `String` |
| `TextArea(id, libellé)` | multiligne, retour à la ligne, barre de défilement | `String` |
| `Choice(id, libellé, options...)` | liste en ligne, ←→ ou initiale | `String` |
| `Bool(id, libellé)` | case, espace ou o/n | `Bool` |
| `Date(id, libellé)` | texte libre + calendrier (ctrl+flèches, pgup/pgdn) | `Moment`, `Time`, `String` |
| `Clock(id, libellé)` | 14h30, 14:30, midi | `String` (15:04) |
| `Duration(id, libellé)` | 1h30, 45min, 3j, 2sem | `Duration` |
| `Durations(id, libellé)` | « 30d, 7d, 1d » | `Durations`, `String` (30d,7d,1d) |
| `Ref(id, libellé, completer)` | complétion, ↑↓ et entrée | `String` |
| `Refs(id, libellé, completer)` | plusieurs valeurs complétées ; entrée ou virgule ajoute, ⌫ sur entrée vide retire la dernière | `Strings`, `String` (a,b) |

Modificateurs : `.Required()`, `.Help(s)`, `.Placeholder(s)`, `.Default(v)`
(string, bool, time.Time, when.Moment, time.Duration, []time.Duration, []string),
`.Clock("09:00")` (heure d'une date seule), `.AllowNone()` (une Date accepte
« aucune », « jamais », « none » : `Values.None(id)`, `String` = « none »), `.From(idDate)` (aperçu des dates
d'un préavis), `.ShowIf(id, valeurs...)`, `.Strict()` (Ref limitée aux
propositions). Un `Refs` obligatoire exige au moins une valeur. Formulaire : `.Validate(func(Values) Errors)`, `.Now(func() time.Time)`.

Un `Choice` obligatoire sans `Default` démarre sans présélection ; un champ
masqué par `ShowIf` n'a pas de valeur.

## Exemple

```go
form := tuikit.NewForm("echeance",
	tuikit.Choice("sphere", "Sphère", "perso", "pro").Required(),
	tuikit.Text("title", "Titre").Required(),
	tuikit.Date("due", "Date").Clock("09:00").Required(),
	tuikit.Durations("notice", "Préavis").Default("30d,7d,1d").From("due"),
	tuikit.Choice("term", "Au terme", "aucune", "message", "agent", "commande"),
	tuikit.TextArea("cmd", "Commande").ShowIf("term", "commande"),
	tuikit.Ref("ref", "Réf.", func(q string) []tuikit.Item { return refs(q) }),
	tuikit.TextArea("msg", "Message"),
)
m.modal = tuikit.NewModal("Nouvelle échéance", form).SetSize(m.width, m.height)

// case tuikit.DoneMsg:
//   msg.Values.String("sphere"), msg.Values.Time("due"), msg.Values.Durations("notice")
```

## Complétion des références

`github.com/aclemen1/tuikit/complete` propose les références à `Ref` et
`Refs` à partir de sources déclarées une fois dans `~/.config/tuikit/refs.yaml`
(ou `$TUIKIT_REFS`, ou `$XDG_CONFIG_HOME/tuikit/refs.yaml`) :

```yaml
sources:
  office:
    run: [office, ls, --status, all, --format, json]
    prefix: "office:"
    value: id
    label: title
    match: [alias, title]    # champs cherchés en plus ; une liste élément par élément
  fixed:
    static:
      - {value: desk, label: le desk}
```

```go
c, err := complete.Load("office", "contact")      // sources nommées du fichier
tuikit.Refs("refs", "Réfs", c.Complete)

s, _ := complete.Lookup("office")                  // même source, sans préfixe
s.Prefix = ""
tuikit.Ref("to", "À", complete.New(s).Complete)
```

| Règle | Effet |
|---|---|
| JSON lu | `{ok, result: [...]}`, `{ok, result: {items: [...]}}` ou `[...]` |
| Commandes | lancées par `New` en arrière-plan, cache de 5 min (`complete.TTL`) partagé dans le processus ; la frappe n'attend jamais |
| Recherche | chaque mot tapé dans value, label ou match, sans casse ni accents ; le préfixe peut être tapé (`office:P-00`) ; un autre préfixe écarte la source |
| Ordre | valeur exacte, puis valeur ou champ match commençant par la saisie, puis le reste |
| Échec | une commande en échec ne propose rien ; `c.Err()` dit pourquoi |

### Configuration d'un outil

Chaque outil liste ses sources par champ, sous la clé `complete:`, avec les noms
de `refs.yaml`. Il ne nomme aucune source dans son code.

```yaml
complete:
  refs: [office, contact, task, due, note, guard, oj]
  to: [{source: office, prefix: ""}]   # « À » de task : l'id seul
  people: [contact]
```

Une entrée est un nom, ou `{source, prefix}` pour changer le préfixe inséré.

```go
type Config struct {
	Complete map[string]complete.Uses `yaml:"complete"`
}

c, missing := complete.FromConfig(cfg.Complete["refs"])
// missing : noms absents de refs.yaml ; les autres sources marchent
tuikit.Refs("refs", "Réfs", c.Complete)
```

`complete.LoadNames(noms)` fait de même pour des noms seuls. `complete.Load`
échoue au premier nom absent ; `FromConfig` et `LoadNames` continuent.

`complete.Source` porte des tags yaml et json : un outil peut aussi déclarer une
source complète et appeler `complete.New(sources...)`.

## Travaux de fond

`tuikit.Busy` montre les travaux de fond d'une TUI : un spinner pendant qu'un
travail tourne, la fin quelques secondes, un échec en rouge jusqu'à ce qu'on
ouvre la liste. Sans travail ni message à montrer, il n'anime rien.

```go
m.busy = tuikit.NewBusy()

cmd := m.busy.Run("confier PT-0012", func() (string, error) {
	return "confiée à P-0019", confier()   // texte de fin, ou erreur
})
cmd := m.busy.Wrap("relecture task", reloadCmd) // son message suit ; une error est un échec

// Update, avant tout le reste :
if cmd, ok := m.busy.Update(msg); ok {
	return m, cmd
}
// touche commune tuikit.BusyKey (« ! ») :
m.modal = tuikit.NewModal("Travaux", m.busy.List()).SetSize(m.width, m.height)

// View, en-tête ou barre d'état :
m.busy.View()
```

| État | `View()` |
|---|---|
| un travail | `⠋ relecture task` |
| plusieurs | `⠋ 2 en cours · confier PT-0012` (le plus ancien) |
| fin | `✓ confiée à P-0019`, 4 s (`tuikit.DoneFor`) |
| échec | `✗ confier PT-0012 : dossier inconnu` en rouge, `(+n)` s'il y en a d'autres, jusqu'à l'ouverture de la liste |
| rien | chaîne vide |

La liste garde les 50 derniers travaux : heure, durée, texte de fin ou erreur
complète. `Running()` et `Unread()` donnent les compteurs.

## Focus partagé

`tuikit.Focus` écrit la sélection d'une TUI dans le fichier de focus commun,
lu par les autres outils : `~/.local/state/office/focus.json`, ou `$OFFICE_FOCUS`.

```go
m.focus = tuikit.NewFocus("task")

// à chaque changement de sélection :
cmd := m.focus.Set("task:PT-0012", "Relancer le notaire", "perso")

// Update, avant tout le reste :
if cmd, ok := m.focus.Update(msg); ok {
	return m, cmd
}
```

```json
{"ref":"task:PT-0012","title":"Relancer le notaire","tool":"task","sphere":"perso","at":"2026-10-12T10:00:00+02:00"}
```

| Règle | Effet |
|---|---|
| Anti-rebond | seule la dernière sélection après 300 ms (`tuikit.FocusDelay`) est écrite |
| Même ref | aucune écriture, aucun tick ; une ref vide est ignorée |
| Écriture | fichier temporaire puis renommage ; répertoire créé si besoin |
| Échec | silencieux |

## Langue

Les textes de tuikit (question d'abandon, Oui/Non, aides, erreurs, lecture des
dates, calendrier) sont en français par défaut. Une TUI en anglais appelle une
fois, au démarrage :

```go
tuikit.SetLanguage(tuikit.English) // règle aussi when
```

La saisie des dates accepte le français et l'anglais quelle que soit la langue
(« demain 14h », « tomorrow at 3pm », « next friday », « in 2 weeks »,
« Nov 12 »). `Values.String` d'un `Bool` reste « oui » ou « non ».

## when

`github.com/aclemen1/tuikit/when` lit les dates, heures et durées :

| Fonction | Lit |
|---|---|
| `Parse(s, now)` | 2026-11-15, 2026-11-15 14:00, RFC 3339, 15.11.2026, 15.11, 15/11/26, 12 novembre, 1er déc., aujourd'hui, demain, après-demain, hier, vendredi, vendredi prochain, ce vendredi, lundi en huit, semaine prochaine, mois prochain, fin du mois, +3j, 3d, 2w, dans 2 semaines, dans 1 mois, +2h, dans 1h30, 14h30 seul ; suivi ou non d'une heure (« demain 14h », « vendredi à 9h30 ») |
| `ParseClock(s)` | 14h30, 14h, 14:30, 9:05, midi, minuit |
| `ParseDuration(s)` | 30min, 2h, 1h30, 3j, 7d, 2sem, 2w, 1j 2h |
| `ParseDurations(s)` | « 30d, 7d, 1d » |
| `Describe(m, now, "09:00")` | « mer. 31.03.2027 (09:00) · dans 173 jours » |

Aussi `Moment`, `FormatDuration`, `Words`, `Before`, `BeforeBy`, `Horizon`,
`Day`, `Until`, repris de `internal/when` de due-cli et task-cli.

Règles : sans année, une date prend sa prochaine occurrence ; « vendredi » et
« vendredi prochain » désignent le premier vendredi après aujourd'hui, « ce
vendredi » l'inclut ; une heure seule désigne aujourd'hui, ou demain si elle est
passée ; « 2h » seul est une heure (02:00), un délai s'écrit « +2h » ou
« dans 2h ».

## Démo

```
go run github.com/aclemen1/tuikit/cmd/tuikit-demo@latest      # -en : en anglais
```
