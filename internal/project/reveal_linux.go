//go:build linux

package project

func platformRevealCommand(directory string) (integrationCommand, error) {
	return revealCommandFor("linux", directory)
}
