package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var ansiEscRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

var enMonths = [...]string{"", "January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December"}

type uiState int

const (
	stateQuestion uiState = iota
	stateEvaluating
	stateResult
	stateDone
)

type evalDoneMsg struct {
	result evalResult
	err    error
}

type model struct {
	deckName  string
	cards     []CardState
	deckTotal int
	idx       int
	state     uiState
	textarea  textarea.Model
	spinner   spinner.Model
	result    *evalResult
	sched     *schedResult
	err       error
	width     int
	height    int
	ev        *evaluator
	st        store
}

func newModel(deckName string, cards []CardState, deckTotal int, ev *evaluator, st store) model {
	ta := textarea.New()
	ta.Placeholder = "Your answer..."
	ta.Focus()
	ta.SetWidth(60)
	ta.SetHeight(4)
	ta.CharLimit = 0
	ta.ShowLineNumbers = false

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorAccent)

	return model{
		deckName:  deckName,
		cards:     cards,
		deckTotal: deckTotal,
		state:     stateQuestion,
		textarea:  ta,
		spinner:   sp,
		ev:        ev,
		st:        st,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textarea.SetWidth(msg.Width - 6)
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case stateQuestion:
			return m.updateQuestion(msg)
		case stateResult:
			return m.updateResult(msg)
		case stateDone:
			if msg.String() == "q" || msg.Type == tea.KeyCtrlC {
				return m, tea.Quit
			}
		}

	case evalDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateResult
			return m, nil
		}
		m.result = &msg.result

		card := m.cards[m.idx]
		sr, err := m.st.submitReview(card.ID, msg.result.correct, msg.result.accuracy, msg.result.keywordsScore)
		if err != nil {
			m.err = err
			m.state = stateResult
			return m, nil
		}
		m.sched = &sr

		if sr.reshowInSession {
			m.cards = append(m.cards, card)
		}

		m.state = stateResult
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	if m.state == stateQuestion {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) updateQuestion(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEnter:
		answer := strings.TrimSpace(m.textarea.Value())
		if answer == "" {
			return m, nil
		}
		card := m.cards[m.idx]
		m.state = stateEvaluating
		return m, tea.Batch(m.spinner.Tick, m.doEvaluate(card.Concept, card.Reference, answer))
	}
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m model) updateResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEnter:
		m.idx++
		if m.idx >= len(m.cards) {
			m.state = stateDone
			return m, nil
		}
		m.state = stateQuestion
		m.result = nil
		m.sched = nil
		m.err = nil
		m.textarea.Reset()
		m.textarea.Focus()
		return m, textarea.Blink
	}
	return m, nil
}

func (m model) doEvaluate(concept, reference, answer string) tea.Cmd {
	return func() tea.Msg {
		result, err := m.ev.evaluate(concept, reference, answer)
		return evalDoneMsg{result: result, err: err}
	}
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder

	remaining := len(m.cards) - m.idx
	if m.state == stateResult {
		remaining = len(m.cards) - m.idx - 1
	} else if m.state == stateDone {
		remaining = 0
	}
	b.WriteString(styleHeader.Render(fmt.Sprintf("flash  •  %s  •  %d/%d", m.deckName, remaining, m.deckTotal)))
	b.WriteString("\n")
	b.WriteString(divider(m.width))
	b.WriteString("\n")

	switch m.state {
	case stateDone:
		return m.viewDone(&b)
	case stateQuestion:
		return m.viewQuestion(&b)
	case stateEvaluating:
		return m.viewEvaluating(&b)
	case stateResult:
		return m.viewResult(&b)
	}
	return b.String()
}

func (m model) viewQuestion(b *strings.Builder) string {
	card := m.cards[m.idx]
	b.WriteString(styleConcept.Render(card.Concept))
	b.WriteString("\n")
	b.WriteString(divider(m.width))
	b.WriteString("\n\n")
	b.WriteString(styleLabel.Render("Your answer:"))
	b.WriteString("\n")
	b.WriteString(m.textarea.View())
	b.WriteString("\n\n")
	b.WriteString(styleMuted.Render("enter to submit  •  ctrl+c to quit"))
	return b.String()
}

func (m model) viewEvaluating(b *strings.Builder) string {
	card := m.cards[m.idx]
	b.WriteString(styleConcept.Render(card.Concept))
	b.WriteString("\n")
	b.WriteString(divider(m.width))
	b.WriteString("\n\n")
	b.WriteString(m.spinner.View())
	b.WriteString(styleAccent.Render(" Evaluating..."))
	b.WriteString("\n")
	return b.String()
}

func (m model) viewResult(b *strings.Builder) string {
	card := m.cards[m.idx]
	b.WriteString(styleConcept.Render(card.Concept))
	b.WriteString("\n")
	b.WriteString(divider(m.width))
	b.WriteString("\n\n")
	b.WriteString(styleLabel.Render("Reference:"))
	b.WriteString("\n")
	ref := card.Reference
	if m.result != nil {
		ref = renderBoldMDEval(card.Reference, m.textarea.Value(), m.ev.cfg.threshold)
	} else {
		ref = renderBoldMD(ref)
	}
	b.WriteString(ansiWordWrap(ref, m.width-4))
	b.WriteString("\n\n")
	b.WriteString(divider(m.width))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(styleError.Render("Error: " + m.err.Error()))
		b.WriteString("\n\n")
		b.WriteString(styleMuted.Render("enter to continue"))
		return b.String()
	}
	if m.result == nil {
		return b.String()
	}

	if m.result.correct {
		b.WriteString(styleSuccess.Render("Correct"))
	} else {
		b.WriteString(styleError.Render("Incorrect"))
	}
	if m.sched != nil {
		if m.sched.reshowInSession {
			b.WriteString(styleMuted.Render("  •  review again this session"))
		} else {
			b.WriteString(styleMuted.Render("  •  next review " + formatDate(m.sched.nextDue)))
		}
	}
	b.WriteString("\n\n")
	b.WriteString(divider(m.width))
	b.WriteString("\n")
	b.WriteString(styleMuted.Render("enter to continue"))
	return b.String()
}

func (m model) viewDone(b *strings.Builder) string {
	b.WriteString("\n")
	b.WriteString(styleSuccess.Render("Session complete!"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("%s cards reviewed\n", styleAccent.Render(fmt.Sprintf("%d", len(m.cards)))))
	b.WriteString("\n")
	b.WriteString(styleMuted.Render("q to quit"))
	return b.String()
}

// renderBoldMD replaces **text** with terminal bold.
func renderBoldMD(md string) string {
	return boldRe.ReplaceAllStringFunc(md, func(match string) string {
		return lipgloss.NewStyle().Bold(true).Inline(true).Render(match[2 : len(match)-2])
	})
}

// renderBoldMDEval replaces **text** with colored keywords:
// present in answer → gray, absent → bold red.
func renderBoldMDEval(md, userAnswer string, threshold float64) string {
	normAnswer := normalizeText(userAnswer)
	return boldRe.ReplaceAllStringFunc(md, func(match string) string {
		kw := match[2 : len(match)-2]
		if partialRatio(normalizeText(kw), normAnswer) >= threshold {
			return lipgloss.NewStyle().Foreground(colorMuted).Bold(true).Inline(true).Render(kw)
		}
		return lipgloss.NewStyle().Foreground(colorError).Bold(true).Inline(true).Render(kw)
	})
}

// ansiWordWrap wraps s at width, preserving existing newlines.
func ansiWordWrap(s string, width int) string {
	if width <= 0 {
		return s
	}
	paragraphs := strings.Split(s, "\n")
	wrapped := make([]string, len(paragraphs))
	for i, p := range paragraphs {
		wrapped[i] = wrapLine(p, width)
	}
	return strings.Join(wrapped, "\n")
}

func wrapLine(s string, width int) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return s
	}
	visLen := func(w string) int {
		return len([]rune(ansiEscRe.ReplaceAllString(w, "")))
	}
	var lines []string
	current := words[0]
	currentLen := visLen(words[0])
	for _, w := range words[1:] {
		wl := visLen(w)
		if currentLen+1+wl > width {
			lines = append(lines, current)
			current = w
			currentLen = wl
		} else {
			current += " " + w
			currentLen += 1 + wl
		}
	}
	return strings.Join(append(lines, current), "\n")
}

func formatDate(t time.Time) string {
	now := time.Now()
	t = t.Local()
	switch {
	case t.Year() == now.Year() && t.YearDay() == now.YearDay():
		return "today"
	case t.Year() == now.Year() && t.YearDay() == now.YearDay()+1:
		return "tomorrow"
	case t.Year() == now.Year():
		return fmt.Sprintf("%s %d", enMonths[t.Month()], t.Day())
	default:
		return fmt.Sprintf("%s %d, %d", enMonths[t.Month()], t.Day(), t.Year())
	}
}
