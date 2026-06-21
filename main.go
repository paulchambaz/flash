package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Manual arg parsing so flags work before or after positional args.
	var reset bool
	var cliDB string
	var rest []string
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-reset" || args[i] == "--reset":
			reset = true
		case args[i] == "-db" || args[i] == "--db":
			i++
			if i < len(args) {
				cliDB = args[i]
			}
		case strings.HasPrefix(args[i], "-db=") || strings.HasPrefix(args[i], "--db="):
			cliDB = strings.SplitN(args[i], "=", 2)[1]
		default:
			rest = append(rest, args[i])
		}
	}

	if len(rest) == 0 {
		printUsage()
		os.Exit(1)
	}

	switch rest[0] {
	case "serve":
		runServeCmd()
	case "push":
		if len(rest) != 2 {
			fmt.Fprintln(os.Stderr, "usage: flash push <host:deck.md>")
			os.Exit(1)
		}
		runPushCmd(rest[1])
	default:
		if len(rest) != 1 {
			printUsage()
			os.Exit(1)
		}
		runStudyCmd(rest[0], reset, cliDB)
	}
}

// parseTarget splits "host:deck.md" → ("host", "deck", true)
// and "deck.md" → ("", "deck", false).
func parseTarget(arg string) (host, deckName string, isRemote bool) {
	if idx := strings.Index(arg, ":"); idx > 0 {
		host = arg[:idx]
		filename := arg[idx+1:]
		deckName = strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
		isRemote = true
		return
	}
	deckName = strings.TrimSuffix(filepath.Base(arg), filepath.Ext(arg))
	return
}

func runServeCmd() {
	cfg := loadConfig("", "")
	if err := runServe(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: serve: %v\n", err)
		os.Exit(1)
	}
}

func runPushCmd(target string) {
	host, deckName, isRemote := parseTarget(target)
	if !isRemote {
		fmt.Fprintln(os.Stderr, "error: push requires <host:deck.md> syntax")
		os.Exit(1)
	}

	// Local file is the filename part after the colon, read from cwd.
	localFile := target[strings.Index(target, ":")+1:]
	content, err := os.ReadFile(localFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read %s: %v\n", localFile, err)
		os.Exit(1)
	}

	cfg := loadConfig("", "")
	if resolved, ok := cfg.Aliases[host]; ok {
		host = resolved
	}
	rs := newRemoteStore(host, deckName, cfg.RemoteToken, cfg.RemotePort)
	if err := rs.pushDeck(content); err != nil {
		fmt.Fprintf(os.Stderr, "error: push: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Pushed %s to %s.\n", localFile, host)
}

func runStudyCmd(arg string, reset bool, cliDB string) {
	host, deckName, isRemote := parseTarget(arg)

	if isRemote {
		runRemoteStudy(host, deckName, reset)
	} else {
		runLocalStudy(arg, deckName, reset, cliDB)
	}
}

func runLocalStudy(deckPath, deckName string, reset bool, cliDB string) {
	cards, err := parseDeck(deckPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(cards) == 0 {
		fmt.Fprintln(os.Stderr, "error: no cards found in deck")
		os.Exit(1)
	}

	cfg := loadConfig(deckPath, cliDB)

	database, err := openDB(cfg.DB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer database.close()

	if reset {
		if err := database.resetDeck(deckName); err != nil {
			fmt.Fprintf(os.Stderr, "error: reset: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Reset %s.\n", deckName)
		return
	}

	if err := database.syncCards(deckName, cards); err != nil {
		fmt.Fprintf(os.Stderr, "error: sync cards: %v\n", err)
		os.Exit(1)
	}

	due, err := database.dueCards(deckName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: due cards: %v\n", err)
		os.Exit(1)
	}
	if len(due) == 0 {
		fmt.Println("Aucune carte à réviser aujourd'hui.")
		return
	}

	total, err := database.deckTotal(deckName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: deck total: %v\n", err)
		os.Exit(1)
	}

	ev := newEvaluator(evalConfigFrom(cfg))
	m := newModel(deckName, due, total, ev, database)
	runTUI(m)
}

func runRemoteStudy(host, deckName string, reset bool) {
	cfg := loadConfig("", "")
	if resolved, ok := cfg.Aliases[host]; ok {
		host = resolved
	}
	if cfg.RemoteToken == "" {
		fmt.Fprintln(os.Stderr, "error: remote_token must be set in flash.cfg or FLASH_REMOTE_TOKEN")
		os.Exit(1)
	}

	rs := newRemoteStore(host, deckName, cfg.RemoteToken, cfg.RemotePort)

	if reset {
		if err := rs.resetDeck(); err != nil {
			fmt.Fprintf(os.Stderr, "error: reset: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Reset %s on %s.\n", deckName, host)
		return
	}

	due, err := rs.dueCards(deckName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: due cards: %v\n", err)
		os.Exit(1)
	}
	if len(due) == 0 {
		fmt.Println("Aucune carte à réviser aujourd'hui.")
		return
	}

	total, err := rs.deckTotal(deckName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: deck total: %v\n", err)
		os.Exit(1)
	}

	ev := newEvaluator(evalConfigFrom(cfg))
	m := newModel(deckName, due, total, ev, rs)
	runTUI(m)
}

func runTUI(m model) {
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  flash [flags] <deck.md>           study local deck
  flash [flags] <host:deck.md>      study remote deck
  flash push <host:deck.md>         push local deck to server
  flash serve                       start API server

Flags:
  -db string    SQLite database path (default: <deck>.db)
  -reset        reset all card states for this deck
`)
}
