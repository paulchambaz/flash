package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Card struct {
	Concept   string
	Reference string
}

func parseDeck(path string) ([]Card, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var cards []Card
	var block []string

	flush := func() {
		if len(block) >= 2 {
			concept := strings.TrimSpace(block[0])
			ref := strings.TrimSpace(strings.Join(block[1:], " "))
			if concept != "" && ref != "" {
				cards = append(cards, Card{Concept: concept, Reference: ref})
			}
		}
		block = nil
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			flush()
		} else {
			block = append(block, line)
		}
	}
	flush()

	return cards, scanner.Err()
}
