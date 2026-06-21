package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorPrimary = lipgloss.Color("12")
	colorMuted   = lipgloss.Color("8")
	colorSuccess = lipgloss.Color("10")
	colorError   = lipgloss.Color("9")
	colorAccent  = lipgloss.Color("14")
	colorBar     = lipgloss.Color("4")
	colorBarBg   = lipgloss.Color("0")

	styleHeader  = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	styleMuted   = lipgloss.NewStyle().Foreground(colorMuted)
	styleAccent  = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	styleConcept = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true).PaddingTop(1).PaddingBottom(1)
	styleLabel   = lipgloss.NewStyle().Foreground(colorMuted)
	styleSuccess = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	styleError   = lipgloss.NewStyle().Foreground(colorError).Bold(true)
	styleBarFg   = lipgloss.NewStyle().Foreground(colorBar)
	styleBarBg   = lipgloss.NewStyle().Foreground(colorBarBg)
	styleDivider = lipgloss.NewStyle().Foreground(colorMuted)
)

func divider(width int) string {
	if width <= 0 {
		width = 60
	}
	return styleDivider.Render(strings.Repeat("─", width))
}

func progressBar(score float64, width int) string {
	filled := max(0, min(width, int(score*float64(width))))
	empty := width - filled
	return styleBarFg.Render(strings.Repeat("█", filled)) + styleBarBg.Render(strings.Repeat("░", empty))
}
