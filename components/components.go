package components

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
