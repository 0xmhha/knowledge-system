// Package refs holds the places a type is named other than a parameter, a
// result or a struct field.
package refs

// Item is the type the others refer to.
type Item struct{ N int }

// Store names Item in its method signatures.
type Store interface {
	Get(id int) Item
	Put(item *Item) error
}

// List is a defined type over a slice of Item.
type List []Item

// Double names Item as its receiver and in its body.
func (i Item) Double() int {
	var j Item = i
	return j.N * 2
}

// Convert names Item only in its body, in a type assertion.
func Convert(x any) int {
	return x.(Item).N
}
