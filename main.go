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
	case "list":
		if len(rest) != 2 {
			fmt.Fprintln(os.Stderr, "usage: flash list <host>")
			os.Exit(1)
		}
		runListCmd(rest[1])
	case "ps":
		if len(rest) != 2 {
			fmt.Fprintln(os.Stderr, "usage: flash ps <host>")
			os.Exit(1)
		}
		runPSCmd(rest[1])
	case "show":
		if len(rest) != 2 {
			fmt.Fprintln(os.Stderr, "usage: flash show <host:deck.md>")
			os.Exit(1)
		}
		runShowCmd(rest[1])
	case "rm":
		if len(rest) != 2 {
			fmt.Fprintln(os.Stderr, "usage: flash rm <host:deck.md>")
			os.Exit(1)
		}
		runRmCmd(rest[1])
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

func runPSCmd(host string) {
	cfg := loadConfig("", "")
	if resolved, ok := cfg.Aliases[host]; ok {
		host = resolved
	}
	rs := newRemoteStore(host, "", cfg.RemoteToken, cfg.RemotePort)
	decks, err := rs.ps()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: ps: %v\n", err)
		os.Exit(1)
	}
	if len(decks) == 0 {
		fmt.Println("No decks loaded.")
		return
	}
	for _, d := range decks {
		fmt.Println(d)
	}
}

func runShowCmd(target string) {
	host, deckName, isRemote := parseTarget(target)
	if !isRemote {
		fmt.Fprintln(os.Stderr, "error: show requires <host:deck.md> syntax")
		os.Exit(1)
	}
	cfg := loadConfig("", "")
	if resolved, ok := cfg.Aliases[host]; ok {
		host = resolved
	}
	rs := newRemoteStore(host, deckName, cfg.RemoteToken, cfg.RemotePort)
	info, err := rs.showDeck()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: show: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("deck:        %s\n", deckName)
	fmt.Printf("total:       %d\n", info.Total)
	fmt.Printf("due:         %d\n", info.Due)
	if info.LastReview != nil {
		fmt.Printf("last review: %s\n", info.LastReview.Local().Format("2006-01-02 15:04"))
	} else {
		fmt.Printf("last review: never\n")
	}
}

func runRmCmd(target string) {
	host, deckName, isRemote := parseTarget(target)
	if !isRemote {
		fmt.Fprintln(os.Stderr, "error: rm requires <host:deck.md> syntax")
		os.Exit(1)
	}
	cfg := loadConfig("", "")
	if resolved, ok := cfg.Aliases[host]; ok {
		host = resolved
	}
	rs := newRemoteStore(host, deckName, cfg.RemoteToken, cfg.RemotePort)
	if err := rs.deleteDeck(); err != nil {
		fmt.Fprintf(os.Stderr, "error: rm: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Deleted %s on %s.\n", deckName, host)
}

func runListCmd(host string) {
	cfg := loadConfig("", "")
	if resolved, ok := cfg.Aliases[host]; ok {
		host = resolved
	}
	rs := newRemoteStore(host, "", cfg.RemoteToken, cfg.RemotePort)
	decks, err := rs.listDecks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: list: %v\n", err)
		os.Exit(1)
	}
	if len(decks) == 0 {
		fmt.Println("No decks on server.")
		return
	}
	for _, d := range decks {
		fmt.Println(d)
	}
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
  flash list <host>                 list decks on server
  flash ps <host>                   list loaded decks on server
  flash show <host:deck.md>         show deck stats
  flash push <host:deck.md>         push local deck to server
  flash rm <host:deck.md>           delete deck from server
  flash serve                       start API server

Flags:
  -db string    SQLite database path (default: <deck>.db)
  -reset        reset all card states for this deck
`)
}
