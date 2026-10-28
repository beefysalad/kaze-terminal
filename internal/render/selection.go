package render

// Point is a grid position.
type Point struct{ Col, Row int }

// Selection is a mouse selection on the grid.
type Selection struct {
	Start, End Point
	Active     bool
}
