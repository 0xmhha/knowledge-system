package flowfix

// submitFromTest is test code that sends to the workers.
func submitFromTest() {
	NewPool().Submit(9)
	viaTestsOnly()
}
