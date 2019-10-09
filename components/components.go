package components

import "math"

// Spatial represents attribuates relating to the physical presence of
// an entity.
type Spatial struct {
	Stackable  bool
	Toggleable bool
}

// Position represents the absolute 2D position of an entity.
type Position struct {
	X int
	Y int
}

func (p Position) Distance(other Position) float64 {
	xDiff := float64(other.X - p.X)
	yDiff := float64(other.Y - p.Y)

	sum := math.Pow(xDiff, 2) + math.Pow(yDiff, 2)
	return math.Sqrt(sum)
}

func (p Position) RoundDistance(other Position) int {
	dist := p.Distance(other)
	rounded := math.Round(dist)

	return int(rounded)
}
