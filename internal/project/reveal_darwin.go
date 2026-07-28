//go:build darwin

package project

func platformRevealCommand(directory string) (integrationCommand, error) {
	return revealCommandFor("darwin", directory)
}
