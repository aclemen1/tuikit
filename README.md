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
