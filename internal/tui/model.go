package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"agent-rules-tui/internal/config"
	"agent-rules-tui/internal/discoverycache"
	"agent-rules-tui/internal/install"
	"agent-rules-tui/internal/scan"
)

const baseListTop = 3

type rowKind int

const (
	rowProject rowKind = iota
	rowFile
)

type row struct {
	kind    rowKind
	project int
	file    int
	text    string
}

type Model struct {
	cfg       config.Config
	projects  []scan.Project
	expanded  map[string]bool
	rows      []row
	selected  int
	offset    int
	width     int
	height    int
	filter    string
	filtering bool
	loading   bool
	opening   bool
	status    string

	pathNotice          *install.Notice
	pathPromptDismissed bool
}

type scanMsg struct {
	projects []scan.Project
	err      error
	cacheErr error
	elapsed  time.Duration
}

type editorDoneMsg struct {
	path string
	err  error
}

type installDoneMsg struct {
	err error
}

type ignorePathPromptDoneMsg struct {
	err error
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	projectStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	fileStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62"))
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	matchStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
)

func New(cfg config.Config, notices ...*install.Notice) Model {
	var notice *install.Notice
	if len(notices) > 0 {
		notice = notices[0]
	}

	m := Model{
		cfg:        cfg,
		expanded:   map[string]bool{},
		loading:    true,
		status:     "Scanning...",
		selected:   0,
		pathNotice: notice,
	}

	if snapshot, ok, err := discoverycache.Load(cfg); ok {
		m.projects = snapshot.Projects
		m.rebuildRows()
		m.status = fmt.Sprintf("Showing cached results from %s; refreshing...", snapshot.SavedAt.Format("Jan 2 15:04"))
	} else if err != nil {
		m.status = "Cache read failed; scanning..."
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return scanCmd(m.cfg)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampSelection()
		return m, nil
	case scanMsg:
		m.loading = false
		if msg.err != nil {
			m.status = "scan failed: " + msg.err.Error()
			return m, nil
		}
		m.projects = msg.projects
		m.rebuildRows()
		m.status = fmt.Sprintf("Found %d files in %d projects in %s", countFiles(m.projects), len(m.projects), msg.elapsed.Round(time.Millisecond))
		if msg.cacheErr != nil {
			m.status += "; cache write failed: " + msg.cacheErr.Error()
		}
		m.clampSelection()
		return m, nil
	case editorDoneMsg:
		m.opening = false
		if msg.err != nil {
			m.status = "editor failed: " + msg.err.Error()
		} else {
			m.status = "Closed " + compactPath(msg.path)
		}
		return m, tea.Batch(tea.EnterAltScreen, tea.EnableMouseCellMotion)
	case installDoneMsg:
		if msg.err != nil {
			m.status = "install failed: " + msg.err.Error()
			return m, nil
		}
		m.pathPromptDismissed = true
		m.status = "Installed agent-rules into ~/.local/bin. Restart your shell if PATH was updated."
		return m, nil
	case ignorePathPromptDoneMsg:
		if msg.err != nil {
			m.status = "could not remember ignore choice: " + msg.err.Error()
			return m, nil
		}
		m.cfg.IgnorePathPrompt = true
		m.pathPromptDismissed = true
		m.status = "Will not remind you about shell PATH again."
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	}
	return m, nil
}

func (m Model) View() string {
	width := m.width
	if width <= 0 {
		width = 100
	}
	height := m.height
	if height <= 0 {
		height = 28
	}

	lines := make([]string, 0, height)
	header := titleStyle.Render("agent-rules") + helpStyle.Render("  click project to expand  click file or enter to open  / filter  r refresh  c config  q quit")
	lines = append(lines, truncate(header, width))

	editorLine := fmt.Sprintf("editor: %s   roots: %s", normalizeEditor(m.cfg.Editor), strings.Join(m.cfg.Roots, ", "))
	lines = append(lines, truncate(helpStyle.Render(editorLine), width))

	filterPrefix := "filter: "
	if m.filtering {
		filterPrefix = "filter* "
	}
	filterLine := filterPrefix + m.filter
	if m.filter == "" {
		filterLine += helpStyle.Render("all")
	}
	lines = append(lines, truncate(filterLine, width))

	if m.showPathPrompt() {
		prompt := "agent-rules is not installed in this shell. a add to ~/.local/bin  i ignore  I remember ignore"
		lines = append(lines, truncate(errorStyle.Render(prompt), width))
	}

	listHeight := height - m.listTop() - 1
	if listHeight < 1 {
		listHeight = 1
	}

	if m.loading && len(m.rows) == 0 {
		lines = append(lines, truncate(statusStyle.Render("Scanning filesystem..."), width))
	} else if len(m.rows) == 0 {
		lines = append(lines, truncate(statusStyle.Render("No agent memory files matched this scan."), width))
	} else {
		m.ensureVisible()
		end := min(len(m.rows), m.offset+listHeight)
		for i := m.offset; i < end; i++ {
			lines = append(lines, m.renderRow(i, width))
		}
	}

	for len(lines) < height-1 {
		lines = append(lines, "")
	}

	status := m.status
	if strings.HasPrefix(status, "scan failed:") || strings.HasPrefix(status, "editor failed:") || strings.HasPrefix(status, "install failed:") {
		status = errorStyle.Render(status)
	} else {
		status = statusStyle.Render(status)
	}
	lines = append(lines, truncate(status, width))

	return strings.Join(lines, "\n")
}

func (m Model) renderRow(index int, width int) string {
	r := m.rows[index]
	prefix := "  "
	style := fileStyle
	if r.kind == rowProject {
		prefix = ""
		style = projectStyle
	}
	if index == m.selected {
		prefix = "> "
	}

	line := prefix + r.text
	if r.kind == rowFile {
		file := m.projects[r.project].Files[r.file]
		line = fmt.Sprintf("%s%s  %s", prefix, file.RelPath, helpStyle.Render(compactPath(filepath.Dir(file.Path))))
	}
	plainLine := truncate(stripANSI(line), width)
	if index == m.selected {
		return selectedStyle.Render(padRight(plainLine, width))
	}
	if m.filter != "" && r.kind == rowFile {
		return truncate(highlightMatch(plainLine, m.filter), width)
	}
	return truncate(style.Render(plainLine), width)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filtering {
		switch msg.Type {
		case tea.KeyEsc, tea.KeyEnter:
			m.filtering = false
			return m, nil
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyBackspace, tea.KeyCtrlH:
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.rebuildRows()
				m.clampSelection()
			}
			return m, nil
		case tea.KeyRunes:
			m.filter += string(msg.Runes)
			m.rebuildRows()
			m.clampSelection()
			return m, nil
		}
		return m, nil
	}

	if m.showPathPrompt() {
		switch msg.String() {
		case "a":
			m.status = "Installing agent-rules into ~/.local/bin..."
			return m, installCmd(*m.pathNotice)
		case "i":
			m.pathPromptDismissed = true
			m.status = "Shell PATH reminder ignored for this session."
			return m, nil
		case "I":
			m.status = "Remembering shell PATH reminder ignore choice..."
			return m, ignorePathPromptCmd(m.cfg.ConfigPath)
		}
	}

	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "pgup", "b":
		m.move(-m.listHeight())
	case "pgdown", "f", " ":
		m.move(m.listHeight())
	case "home", "g":
		m.selected = 0
	case "end", "G":
		m.selected = max(0, len(m.rows)-1)
	case "enter", "o":
		return m.openSelected()
	case "r":
		m.loading = true
		m.status = "Refreshing..."
		return m, scanCmd(m.cfg)
	case "/":
		m.filtering = true
	case "backspace":
		if m.filter != "" {
			m.filter = ""
			m.rebuildRows()
			m.clampSelection()
		}
	case "c":
		return m.openPath(m.cfg.ConfigPath)
	}
	m.clampSelection()
	return m, nil
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.offset = max(0, m.offset-3)
		return m, nil
	case tea.MouseButtonWheelDown:
		m.offset = min(max(0, len(m.rows)-1), m.offset+3)
		return m, nil
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		if msg.Y < m.listTop() {
			return m, nil
		}
		index := m.offset + msg.Y - m.listTop()
		if index < 0 || index >= len(m.rows) {
			return m, nil
		}
		m.selected = index
		m.clampSelection()
		switch m.rows[m.selected].kind {
		case rowProject:
			m.toggleProject(m.rows[m.selected].project)
			m.rebuildRows()
			m.clampSelection()
			return m, nil
		case rowFile:
			return m.openSelected()
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) rebuildRows() {
	rows := make([]row, 0)
	filter := strings.ToLower(strings.TrimSpace(m.filter))
	for pi, project := range m.projects {
		startLen := len(rows)
		expanded := m.expanded[projectKey(project)]
		rows = append(rows, row{kind: rowProject, project: pi, text: projectLabel(project, expanded || filter != "")})
		if filter == "" && !expanded {
			continue
		}
		for fi, file := range project.Files {
			target := strings.ToLower(file.RelPath + " " + file.Path + " " + project.Name)
			if filter == "" || strings.Contains(target, filter) {
				rows = append(rows, row{kind: rowFile, project: pi, file: fi, text: file.RelPath})
			}
		}
		if len(rows) == startLen+1 && filter != "" {
			rows = rows[:startLen]
		}
	}
	m.rows = rows
}

func (m *Model) move(delta int) {
	if len(m.rows) == 0 {
		return
	}
	next := m.selected
	for {
		next += delta
		if next < 0 {
			next = 0
			break
		}
		if next >= len(m.rows) {
			next = len(m.rows) - 1
			break
		}
		if delta == 0 {
			break
		}
		break
	}
	m.selected = next
}

func (m *Model) clampSelection() {
	if len(m.rows) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	if m.selected >= len(m.rows) {
		m.selected = len(m.rows) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
	m.ensureVisible()
}

func (m *Model) ensureVisible() {
	listHeight := m.listHeight()
	if listHeight <= 0 {
		return
	}
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+listHeight {
		m.offset = m.selected - listHeight + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m Model) listHeight() int {
	if m.height == 0 {
		return 20
	}
	return max(1, m.height-m.listTop()-1)
}

func (m Model) listTop() int {
	if m.showPathPrompt() {
		return baseListTop + 1
	}
	return baseListTop
}

func (m Model) showPathPrompt() bool {
	return m.pathNotice != nil && !m.pathPromptDismissed && !m.cfg.IgnorePathPrompt
}

func (m Model) openSelected() (tea.Model, tea.Cmd) {
	if len(m.rows) == 0 || m.selected < 0 || m.selected >= len(m.rows) {
		return m, nil
	}
	r := m.rows[m.selected]
	if r.kind == rowProject {
		m.toggleProject(r.project)
		m.rebuildRows()
		m.clampSelection()
		return m, nil
	}
	file := m.projects[r.project].Files[r.file]
	return m.openPath(file.Path)
}

func (m Model) openPath(path string) (tea.Model, tea.Cmd) {
	if m.opening || strings.TrimSpace(path) == "" {
		return m, nil
	}
	m.opening = true
	m.status = "Opening " + compactPath(path)
	return m, openEditorCmd(m.cfg.Editor, path)
}

func (m *Model) toggleProject(projectIndex int) {
	if projectIndex < 0 || projectIndex >= len(m.projects) {
		return
	}
	key := projectKey(m.projects[projectIndex])
	m.expanded[key] = !m.expanded[key]
}

func scanCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		projects, err := scan.Discover(context.Background(), cfg)
		var cacheErr error
		if err == nil {
			cacheErr = discoverycache.Save(cfg, projects)
		}
		return scanMsg{projects: projects, err: err, cacheErr: cacheErr, elapsed: time.Since(start)}
	}
}

func installCmd(notice install.Notice) tea.Cmd {
	return func() tea.Msg {
		return installDoneMsg{err: install.Add(notice)}
	}
}

func ignorePathPromptCmd(configPath string) tea.Cmd {
	return func() tea.Msg {
		return ignorePathPromptDoneMsg{err: config.SetIgnorePathPrompt(configPath, true)}
	}
}

func openEditorCmd(editor string, path string) tea.Cmd {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	cmd := editorCommand(editor, path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorDoneMsg{path: path, err: err}
	})
}

func editorCommand(editor string, path string) *exec.Cmd {
	editor = normalizeEditor(editor)
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return exec.Command(shell, "-lc", editor+" "+shellQuote(path))
}

func normalizeEditor(editor string) string {
	editor = strings.TrimSpace(editor)
	switch strings.ToLower(editor) {
	case "":
		return "nvim"
	case "neovim":
		return "nvim"
	case "vscode":
		return "code -w"
	case "vscode-insiders":
		return "code-insiders -w"
	default:
		return editor
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func countFiles(projects []scan.Project) int {
	count := 0
	for _, project := range projects {
		count += len(project.Files)
	}
	return count
}

func projectLabel(project scan.Project, expanded bool) string {
	marker := "+ "
	if expanded {
		marker = "- "
	}
	if project.Global {
		return fmt.Sprintf("%sGlobal (%d)", marker, len(project.Files))
	}
	return fmt.Sprintf("%s%s (%d)  %s", marker, project.Name, len(project.Files), compactPath(project.Root))
}

func projectKey(project scan.Project) string {
	if project.Global {
		return "global"
	}
	return project.Root
}

func compactPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		if path == home {
			return "~"
		}
		if strings.HasPrefix(path, home+string(os.PathSeparator)) {
			return "~" + strings.TrimPrefix(path, home)
		}
	}
	return path
}

func highlightMatch(line string, filter string) string {
	lower := strings.ToLower(line)
	filter = strings.ToLower(filter)
	index := strings.Index(lower, filter)
	if index < 0 {
		return fileStyle.Render(line)
	}
	before := fileStyle.Render(line[:index])
	match := matchStyle.Render(line[index : index+len(filter)])
	after := fileStyle.Render(line[index+len(filter):])
	return before + match + after
}

func truncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	plain := stripANSI(s)
	if len([]rune(plain)) <= width {
		return s
	}
	runes := []rune(plain)
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func padRight(s string, width int) string {
	if width <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(runes))
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch != 0x1b {
			b.WriteByte(ch)
			continue
		}

		if i+1 >= len(s) {
			break
		}
		i++
		if s[i] == '[' {
			for i+1 < len(s) {
				i++
				if s[i] >= 0x40 && s[i] <= 0x7e {
					break
				}
			}
			continue
		}

		for i+1 < len(s) {
			i++
			if s[i] >= 0x40 && s[i] <= 0x5f {
				break
			}
		}
	}
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
