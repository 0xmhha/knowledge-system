package p

// A second init in another file.
func init() {
	f2()
	_ = B{}
}

func f2() {}

// B is built only by the init in p2.go.
type B struct{}
