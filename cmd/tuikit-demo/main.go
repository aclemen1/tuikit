// Command tuikit-demo tries the components of tuikit over a dummy TUI view.
package main

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/aclemen1/tuikit"
)

var dossiers = []tuikit.Item{
	{Value: "P-0019", Label: "Développement d'office"},
	{Value: "P-0040", Label: "Outil générique d'échéances"},
	{Value: "P-0049", Label: "tuikit"},
	{Value: "PT-0007", Label: "Relancer la gérance"},
}

func complete(q string) []tuikit.Item {
	var out []tuikit.Item
	for _, it := range dossiers {
		if strings.Contains(strings.ToLower(it.Value+" "+it.Label), strings.ToLower(q)) {
			out = append(out, it)
		}
	}
	return out
}

type model struct {
	modal         *tuikit.Modal
	width, height int
	log           []string
}

func (m model) Init() tea.Cmd { return tea.RequestBackgroundColor }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.BackgroundColorMsg:
		tuikit.SetDarkBackground(msg.IsDark())
	case tuikit.DoneMsg:
		m.log = append(m.log, fmt.Sprintf("%s validé : %v", msg.ID, msg.Values))
		return m, nil
	case tuikit.CancelMsg:
		m.log = append(m.log, msg.ID+" annulé")
		return m, nil
	}
	if m.modal.Open() {
		return m, m.modal.Update(msg)
	}
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	var c tuikit.Content
	switch k.String() {
	case "q":
		return m, tea.Quit
	case "c":
		c = tuikit.NewForm("echeance",
			tuikit.Choice("sphere", "Sphère", "perso", "pro").Required(),
			tuikit.Text("title", "Titre").Required(),
			tuikit.Date("due", "Date").Clock("09:00").Required(),
			tuikit.Durations("notice", "Préavis").Default("30d,7d,1d").From("due"),
			tuikit.Choice("term", "Au terme", "aucune", "message", "agent", "commande"),
			tuikit.TextArea("cmd", "Commande").ShowIf("term", "commande"),
			tuikit.Choice("channel", "Canal", "auto", "mail", "tell", "push"),
			tuikit.Ref("ref", "Réf.", complete).Help("dossier ou tâche"),
			tuikit.TextArea("msg", "Message"),
		)
		m.modal = tuikit.NewModal("Nouvelle échéance", c)
	case "n":
		c = tuikit.NewForm("dossier",
			tuikit.Text("title", "Titre").Required(),
			tuikit.TextArea("instr", "Instruction").Help("ce que l'agent doit faire"),
			tuikit.Choice("office", "Office", "perso", "pro").Default("perso"),
			tuikit.Ref("parent", "Dossier parent", complete).Strict(),
			tuikit.Clock("at", "Heure"),
			tuikit.Duration("len", "Durée"),
			tuikit.Bool("start", "Démarrer").Default(true),
		)
		m.modal = tuikit.NewModal("Nouveau dossier", c)
	case "e":
		m.modal = tuikit.NewModal("Clôture de P-0049", tuikit.NewEditor("result", "Résultat", ""))
	case "o":
		m.modal = tuikit.NewModal("Abandon", tuikit.NewConfirm("drop", "Abandonner PE-0007 ?"))
	case "d":
		m.modal = tuikit.NewModal("Suppression", tuikit.NewConfirmTyped("rm", "Supprimer le dossier P-0049 ?", "P-0049"))
	default:
		return m, nil
	}
	m.modal.SetSize(m.width, m.height)
	return m, nil
}

func (m model) View() tea.View {
	lines := []string{"tuikit-demo : c échéance · n dossier · e éditeur · o Oui/Non · d suppression · q quitter", ""}
	lines = append(lines, m.log...)
	v := tea.NewView(tuikit.Overlay(strings.Join(lines, "\n"), m.modal, m.width, m.height))
	v.AltScreen = true
	return v
}

func main() {
	if _, err := tea.NewProgram(model{width: 80, height: 24}).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
