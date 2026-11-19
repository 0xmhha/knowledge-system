package coll1

// Two init functions in two files share the canonical id <pkg>.init; each
// one's statements and value uses must stay with its own node.

var initA int

func init() {
	if initA == 0 {
		initA = load(1)
	}
}
