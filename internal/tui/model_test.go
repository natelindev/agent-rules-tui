package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"agent-rules-tui/internal/config"
	"agent-rules-tui/internal/scan"
)

func TestProjectsAreCollapsedByDefaultAndToggle(t *testing.T) {
	m := testModel()

	if len(m.rows) != 1 {
		t.Fatalf("collapsed rows = %d, want 1", len(m.rows))
	}
	if m.rows[0].kind != rowProject {
		t.Fatalf("first row kind = %v, want project", m.rows[0].kind)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if len(m.rows) != 3 {
		t.Fatalf("expanded rows = %d, want 3", len(m.rows))
	}
	if !strings.HasPrefix(m.rows[0].text, "- ") {
		t.Fatalf("expanded project label = %q, want '-' prefix", m.rows[0].text)
	}
}

func TestMouseProjectClickIgnoresReleaseAndMotion(t *testing.T) {
	m := testModel()
	m.height = 20
	m.width = 100

	updated, _ := m.Update(tea.MouseMsg{
		X:      0,
		Y:      m.listTop(),
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	m = updated.(Model)
	if len(m.rows) != 3 {
		t.Fatalf("rows after press = %d, want expanded rows", len(m.rows))
	}

	for _, msg := range []tea.MouseMsg{
		{X: 0, Y: m.listTop(), Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease},
		{X: 0, Y: m.listTop(), Button: tea.MouseButtonNone, Action: tea.MouseActionRelease},
		{X: 0, Y: m.listTop(), Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion},
	} {
		updated, _ = m.Update(msg)
		m = updated.(Model)
		if len(m.rows) != 3 {
			t.Fatalf("rows after %v = %d, want still expanded", msg, len(m.rows))
		}
	}
}

func testModel() Model {
	m := New(config.Default())
	m.projects = []scan.Project{
		{
			Name: "app",
			Root: "/tmp/app",
			Files: []scan.AgentFile{
				{RelPath: "AGENTS.md", Path: "/tmp/app/AGENTS.md"},
				{RelPath: "CLAUDE.md", Path: "/tmp/app/CLAUDE.md"},
			},
		},
	}
	m.rebuildRows()
	return m
}
