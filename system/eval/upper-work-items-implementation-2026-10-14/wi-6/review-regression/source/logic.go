package fixture

import "os/exec"

type Box struct{ Total int }

func Repeat(command string, values []int) {
	for range values {
		exec.Command(command)
	}
}
func Process(box *Box, command string, values []int, strict bool) *exec.Cmd {
	total := 0
	for _, value := range values {
		total += value
	}
	box.Total = total
	if strict && total < 0 {
		command = "fixed"
	}
	return exec.Command(command)
}
