//go:build !darwin && !linux && !windows

package project

import "runtime"

func platformRevealCommand(directory string) (integrationCommand, error) {
	return revealCommandFor(runtime.GOOS, directory)
}
