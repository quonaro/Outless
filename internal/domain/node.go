package domain

// Node represents a proxy endpoint managed by Outless.
type Node struct {
	ID      string
	URL     string
	GroupID string
	Country string
}
