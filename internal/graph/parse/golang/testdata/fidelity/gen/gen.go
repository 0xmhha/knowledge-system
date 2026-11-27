// Package gen holds the type shapes that mislead a name-based field lookup.
package gen

// Tuple is a package-level struct.
type Tuple struct {
	Address string
	Keys    []string
}

// Marshal declares a local struct with the package type's name, as gencodec
// output does, and writes the local struct's field.
func (t Tuple) Marshal() string {
	type Tuple struct {
		Address string
	}
	var enc Tuple
	enc.Address = t.Address
	return enc.Address
}

// Query is a package-level struct.
type Query struct {
	From int
}

// Criteria is a defined type over Query: its fields are Query's fields.
type Criteria Query

// Set writes Query.From through a Criteria value.
func (c *Criteria) Set(v int) { c.From = v }
