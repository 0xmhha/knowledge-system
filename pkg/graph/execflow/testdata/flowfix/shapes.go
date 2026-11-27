package flowfix

// Shape is measured through an interface call.
type Shape interface {
	Area() int
}

// Square is one implementation of Shape.
type Square struct {
	side int
}

// Area of a square.
func (s Square) Area() int {
	return s.side * s.side
}

// Rect is the other implementation of Shape.
type Rect struct {
	w, h int
}

// Area of a rectangle.
func (r Rect) Area() int {
	return r.w * r.h
}

// Sum adds the areas through the interface.
func Sum(shapes []Shape) int {
	total := 0
	for _, s := range shapes {
		total += s.Area()
	}
	return total
}

// Draw passes both shapes, so the value flow reaches both implementations.
func Draw() int {
	return Sum([]Shape{Square{side: 2}, Rect{w: 2, h: 3}})
}

// Depth calls itself.
func Depth(n int) int {
	if n == 0 {
		return 0
	}
	return Depth(n-1) + 1
}
