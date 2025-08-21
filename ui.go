package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
)

type UiCheckbox struct {
	Cursor   string
	Selected string
	Clear    string
}

type UiModel struct {
	Paginator     paginator.Model
	Items         []string
	Cursor        uint8
	Choices       []string
	CheckboxStyle UiCheckbox
}

func newModel(chats []string, cfg *Config) UiModel {
	p := paginator.New()
	p.PerPage = cfg.General.ContactsPerPage
	p.SetTotalPages(len(chats))

	return UiModel{
		Paginator: p,
		Items:     chats,
		Choices:   make([]string, len(chats)),
		CheckboxStyle: UiCheckbox{
			Cursor:   cfg.Misc.Cursor,
			Selected: cfg.Misc.CheckboxSelected,
			Clear:    cfg.Misc.CheckboxClear,
		},
	}
}

func (m UiModel) Init() tea.Cmd {
	return nil
}

func (m UiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	start, end := m.Paginator.GetSliceBounds(len(m.Items))
	perPage := m.Paginator.PerPage
	limit := end - start
	choices := m.Choices[start:end]
	items := m.Items[start:end]

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "enter":
			return m, tea.Quit
		case " ":
			if choices[m.Cursor] == "" {
				choices[m.Cursor] = items[m.Cursor]
			} else {
				choices[m.Cursor] = ""
			}
		case "down", "j":
			m.Cursor++

			if m.Cursor >= uint8(limit) {
				m.Paginator.NextPage()
				m.Cursor = 0
			}
		case "up", "k":
			m.Cursor--

			if m.Cursor == ^uint8(0) {
				m.Paginator.PrevPage()
				m.Cursor = uint8(perPage) - 1
			}
		}
	}

	m.Paginator, cmd = m.Paginator.Update(msg)

	return m, cmd
}

func (m UiModel) View() string {
	s := strings.Builder{}

	start, end := m.Paginator.GetSliceBounds(len(m.Items))
	limit := m.Paginator.PerPage
	items := m.Items[start:end]
	choices := m.Choices[start:end]

	s.WriteString("Send to...\n\n")

	for i, item := range items {
		if m.Cursor == uint8(i) {
			s.WriteString(m.CheckboxStyle.Cursor + " ")
		} else if choices[i] != "" {
			s.WriteString(m.CheckboxStyle.Selected + " ")
		} else {
			s.WriteString(m.CheckboxStyle.Clear + " ")
		}

		s.WriteString(item)
		s.WriteString("\n")
	}

	if len(items) < limit {
		for range limit - len(items) {
			s.WriteString("\n")
		}
	}

	currentPage := m.Paginator.Page + 1
	pageCount := m.Paginator.TotalPages

	paginationString := fmt.Sprintf("\n[%d/%d]", currentPage, pageCount)

	s.WriteString(paginationString)
	s.WriteString("\n\n h/l ←/→: page • esc q: quit • j/k ↑/↓: item\n")

	return s.String()
}
