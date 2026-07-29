package lib_test

import (
	"testing"

	"example.com/fixture/lib"
)

func TestAnswerExternal(t *testing.T) {
	if lib.Answer() != 42 {
		t.Fatal("wrong answer")
	}
}
