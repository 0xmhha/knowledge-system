//go:build !linux

package lib

func platform() string { return "other" }
