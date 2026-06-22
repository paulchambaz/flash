package main

type store interface {
	dueCards(deck string) ([]CardState, error)
	deckTotal(deck string) (int, error)
	submitReview(cardID int64, correct bool, accuracy, keywordsScore float64) (schedResult, error)
	close() error
}
