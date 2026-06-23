package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// clif holds CLI flag overrides; applied last in loadConfig (highest priority).
var clif struct {
	config      string
	db          string
	ollamaURL   string
	ollamaUser  string
	ollamaPass  string
	model       string
	threshold string
	pace      string
	serveHost string
	servePort   string
	serveToken  string
	serveData   string
	remotePort  string
	remoteToken string
}

func main() {
	// Manual arg parsing: flags accepted before or after positional args.
	flagDests := map[string]*string{
		"config":       &clif.config,
		"db":           &clif.db,
		"ollama-url":   &clif.ollamaURL,
		"ollama-user":  &clif.ollamaUser,
		"ollama-pass":  &clif.ollamaPass,
		"model":        &clif.model,
		"threshold":    &clif.threshold,
		"pace":         &clif.pace,
		"serve-host":   &clif.serveHost,
		"serve-port":   &clif.servePort,
		"serve-token":  &clif.serveToken,
		"serve-data":   &clif.serveData,
		"remote-port":  &clif.remotePort,
		"remote-token": &clif.remoteToken,
	}
	var rest []string
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		matched := false
		for name, dest := range flagDests {
			if args[i] == "-"+name || args[i] == "--"+name {
				if i+1 < len(args) {
					i++
					*dest = args[i]
				}
				matched = true
				break
			}
			if v, ok := strings.CutPrefix(args[i], "-"+name+"="); ok {
				*dest = v
				matched = true
				break
			}
			if v, ok := strings.CutPrefix(args[i], "--"+name+"="); ok {
				*dest = v
				matched = true
				break
			}
		}
		if !matched {
			rest = append(rest, args[i])
		}
	}

	if len(rest) == 0 {
		printUsage()
		os.Exit(1)
	}

	switch rest[0] {
	case "help":
		printUsage()
		os.Exit(0)
	case "serve":
		runServeCmd()
	case "push":
		if len(rest) != 3 {
			fmt.Fprintln(os.Stderr, "usage: flash push <deck.md> <server:> | <server:deck.md>")
			os.Exit(1)
		}
		runPushCmd(rest[1], rest[2])
	case "list":
		if len(rest) != 2 {
			fmt.Fprintln(os.Stderr, "usage: flash list <host>")
			os.Exit(1)
		}
		runListCmd(rest[1])
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
	case "pull":
		if len(rest) != 3 {
			fmt.Fprintln(os.Stderr, "usage: flash pull <server:deck.md> <.> | <deck.md>")
			os.Exit(1)
		}
		runPullCmd(rest[1], rest[2])
	case "stats":
		if len(rest) != 2 {
			fmt.Fprintln(os.Stderr, "usage: flash stats <deck.md>|<host:deck.md>")
			os.Exit(1)
		}
		runStatsCmd(rest[1])
	case "reset":
		if len(rest) != 2 {
			fmt.Fprintln(os.Stderr, "usage: flash reset <deck.md>|<host:deck.md>")
			os.Exit(1)
		}
		runResetCmd(rest[1])
	default:
		if len(rest) != 1 {
			printUsage()
			os.Exit(1)
		}
		runStudyCmd(rest[0])
	}
}

// parseTarget splits "host:deck.md" → ("host", "deck", true)
// and "deck.md" → ("", "deck", false).
func parseTarget(arg string) (host, deckName string, isRemote bool) {
	if idx := strings.Index(arg, ":"); idx > 0 {
		host = arg[:idx]
		filename := arg[idx+1:]
		if filename != "" {
			deckName = strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
		}
		isRemote = true
		return
	}
	deckName = strings.TrimSuffix(filepath.Base(arg), filepath.Ext(arg))
	return
}

func runShowCmd(target string) {
	host, deckName, isRemote := parseTarget(target)
	if !isRemote {
		fmt.Fprintln(os.Stderr, "error: show requires <host:deck.md> syntax")
		os.Exit(1)
	}
	cfg := loadConfig("")
	if resolved, ok := cfg.Aliases[host]; ok {
		host = resolved
	}
	rs := newRemoteStore(host, deckName, cfg.RemoteToken, cfg.RemotePort, cfg.Pace)
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
	cfg := loadConfig("")
	if resolved, ok := cfg.Aliases[host]; ok {
		host = resolved
	}
	rs := newRemoteStore(host, deckName, cfg.RemoteToken, cfg.RemotePort, cfg.Pace)
	if err := rs.deleteDeck(); err != nil {
		fmt.Fprintf(os.Stderr, "error: rm: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Deleted %s on %s.\n", deckName, host)
}

func runListCmd(host string) {
	cfg := loadConfig("")
	if resolved, ok := cfg.Aliases[host]; ok {
		host = resolved
	}
	rs := newRemoteStore(host, "", cfg.RemoteToken, cfg.RemotePort, cfg.Pace)
	items, err := rs.listDecks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: list: %v\n", err)
		os.Exit(1)
	}
	if len(items) == 0 {
		fmt.Println("No decks on server.")
		return
	}
	maxLen := 0
	for _, d := range items {
		if len(d.Name) > maxLen {
			maxLen = len(d.Name)
		}
	}
	for _, d := range items {
		fmt.Printf("%-*s  %d/%d\n", maxLen, d.Name, d.Due, d.Total)
	}
}

func runServeCmd() {
	cfg := loadConfig("")
	if err := runServe(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: serve: %v\n", err)
		os.Exit(1)
	}
}

func runPushCmd(src, dest string) {
	idx := strings.Index(dest, ":")
	if idx <= 0 {
		fmt.Fprintln(os.Stderr, "error: push destination must be <server:> or <server:deck.md>")
		os.Exit(1)
	}
	host := dest[:idx]
	remotepart := dest[idx+1:]

	var deckName string
	if remotepart == "" {
		deckName = strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	} else {
		deckName = strings.TrimSuffix(filepath.Base(remotepart), filepath.Ext(remotepart))
	}

	content, err := os.ReadFile(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read %s: %v\n", src, err)
		os.Exit(1)
	}

	cfg := loadConfig("")
	if resolved, ok := cfg.Aliases[host]; ok {
		host = resolved
	}
	rs := newRemoteStore(host, deckName, cfg.RemoteToken, cfg.RemotePort, cfg.Pace)
	if err := rs.pushDeck(content); err != nil {
		fmt.Fprintf(os.Stderr, "error: push: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Pushed %s to %s:%s.\n", src, host, deckName)
}

func runResetCmd(arg string) {
	host, deckName, isRemote := parseTarget(arg)
	if isRemote {
		cfg := loadConfig("")
		if resolved, ok := cfg.Aliases[host]; ok {
			host = resolved
		}
		rs := newRemoteStore(host, deckName, cfg.RemoteToken, cfg.RemotePort, cfg.Pace)
		if err := rs.resetDeck(); err != nil {
			fmt.Fprintf(os.Stderr, "error: reset: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Reset %s on %s.\n", deckName, host)
	} else {
		cfg := loadConfig(arg)
		db, err := openDB(cfg.DB, cfg.Pace)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer db.close()
		if err := db.resetDeck(deckName); err != nil {
			fmt.Fprintf(os.Stderr, "error: reset: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Reset %s.\n", deckName)
	}
}

func runStudyCmd(arg string) {
	host, deckName, isRemote := parseTarget(arg)

	if isRemote {
		runRemoteStudy(host, deckName)
	} else {
		runLocalStudy(arg, deckName)
	}
}

func runLocalStudy(deckPath, deckName string) {
	cards, err := parseDeck(deckPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(cards) == 0 {
		fmt.Fprintln(os.Stderr, "error: no cards found in deck")
		os.Exit(1)
	}

	cfg := loadConfig(deckPath)

	database, err := openDB(cfg.DB, cfg.Pace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer database.close()

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
		fmt.Println("No cards due today.")
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

func runRemoteStudy(host, deckName string) {
	cfg := loadConfig("")
	if resolved, ok := cfg.Aliases[host]; ok {
		host = resolved
	}
	if cfg.RemoteToken == "" {
		fmt.Fprintln(os.Stderr, "error: remote_token must be set in flash.cfg or FLASH_REMOTE_TOKEN")
		os.Exit(1)
	}

	rs := newRemoteStore(host, deckName, cfg.RemoteToken, cfg.RemotePort, cfg.Pace)

	due, err := rs.dueCards(deckName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: due cards: %v\n", err)
		os.Exit(1)
	}
	if len(due) == 0 {
		fmt.Println("No cards due today.")
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

func runPullCmd(src, dest string) {
	idx := strings.Index(src, ":")
	if idx <= 0 {
		fmt.Fprintln(os.Stderr, "error: pull source must be <server:deck.md>")
		os.Exit(1)
	}
	host := src[:idx]
	remoteFile := src[idx+1:]
	if remoteFile == "" {
		fmt.Fprintln(os.Stderr, "error: pull source must specify a deck: <server:deck.md>")
		os.Exit(1)
	}
	deckName := strings.TrimSuffix(filepath.Base(remoteFile), filepath.Ext(remoteFile))

	var localFile string
	if dest == "." {
		localFile = deckName + ".md"
	} else {
		localFile = dest
	}

	cfg := loadConfig("")
	if resolved, ok := cfg.Aliases[host]; ok {
		host = resolved
	}
	rs := newRemoteStore(host, deckName, cfg.RemoteToken, cfg.RemotePort, cfg.Pace)
	content, err := rs.pullDeck()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: pull: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(localFile, content, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error: write %s: %v\n", localFile, err)
		os.Exit(1)
	}
	fmt.Printf("Pulled %s from %s to %s.\n", deckName, host, localFile)
}

func runStatsCmd(arg string) {
	host, deckName, isRemote := parseTarget(arg)

	if isRemote {
		runRemoteStats(host, deckName)
	} else {
		runLocalStats(arg, deckName)
	}
}

func runLocalStats(deckPath, deckName string) {
	cfg := loadConfig(deckPath)
	db, err := openDB(cfg.DB, cfg.Pace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer db.close()
	printStats(deckName, db)
}

func runRemoteStats(host, deckName string) {
	cfg := loadConfig("")
	if resolved, ok := cfg.Aliases[host]; ok {
		host = resolved
	}
	rs := newRemoteStore(host, deckName, cfg.RemoteToken, cfg.RemotePort, cfg.Pace)
	if deckName == "" {
		activity, err := rs.activity()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: activity: %v\n", err)
			os.Exit(1)
		}
		printActivity(activity)
		return
	}
	stats, err := rs.deckStats()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: stats: %v\n", err)
		os.Exit(1)
	}
	printDeckStats(deckName, stats)
}

func printActivity(days []DayActivity) {
	for _, d := range days {
		if d.Due == 0 && d.Done == 0 {
			continue
		}
		fmt.Printf("%s  %d/%d\n", d.Date, d.Done, d.Due)
	}
}

func printStats(deckName string, db *DB) {
	stats, err := db.deckStats(deckName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: stats: %v\n", err)
		os.Exit(1)
	}
	printDeckStats(deckName, stats)
}

func printDeckStats(deckName string, s DeckStats) {
	fmt.Printf("deck:          %s\n", deckName)
	fmt.Printf("total:         %d\n", s.Total)
	fmt.Printf("new:           %d\n", s.New)
	fmt.Printf("due today:     %d\n", s.Due)
	if s.ReviewCount > 0 {
		fmt.Printf("success rate:  %.0f%%  (%d last reviews)\n", s.SuccessRate*100, s.ReviewCount)
	} else {
		fmt.Printf("success rate:  —\n")
	}
	if s.AvgStability > 0 {
		fmt.Printf("avg stability: %.1f days\n", s.AvgStability)
	} else {
		fmt.Printf("avg stability: —\n")
	}
	if s.LastReview != nil {
		fmt.Printf("last review:   %s\n", s.LastReview.Local().Format("2006-01-02 15:04"))
	} else {
		fmt.Printf("last review:   never\n")
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  flash <deck|host:deck>              study a deck
  flash reset <deck|host:deck>        reset all card states
  flash stats <deck|host:deck>        show deck statistics
  flash stats <host:>                 show yearly activity (done/due per day)
  flash list <host>                   list decks on server
  flash show <host:deck>              show deck info
  flash push <src> <host:[dst]>       push deck to server
  flash pull <host:src> <dst|.>       pull deck from server
  flash rm <host:deck>                delete deck from server
  flash serve                         start API server

Flags:
  -ollama-url <url>     Ollama API endpoint
  -ollama-user <str>    HTTP basic auth username
  -ollama-pass <str>    HTTP basic auth password
  -model <name>         model name (default: qwen3:4b-...)
  -threshold <0-1>      keyword match threshold (default: 0.7)
  -pace <duration>      maximum review interval (default: 7d)
  -db <path>            SQLite path (default: ~/.local/share/flash/<deck>.db)
  -serve-host <addr>    bind address (default: 0.0.0.0)
  -serve-port <int>     listen port (default: 8765)
  -serve-token <str>    API auth token
  -serve-data <dir>     data directory (default: .)
  -remote-port <int>    remote port (default: 443)
  -remote-token <str>   remote auth token
  -config <path>        config file (default: flash.cfg)
`)
}
