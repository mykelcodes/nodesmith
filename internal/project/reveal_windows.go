//go:build windows

package project

func platformRevealCommand(directory string) (integrationCommand, error) {
	return revealCommandFor("windows", directory)
}
