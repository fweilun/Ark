package aiusage

import "errors"

// ErrInsufficientTokens is returned when a user has no tokens remaining for the current month.
var ErrInsufficientTokens = errors.New("insufficient tokens")

// DefaultTokens is the number of tokens granted per month.
const DefaultTokens = 100
