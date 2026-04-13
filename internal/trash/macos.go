package trash

import (
	"context"
	"os/exec"
)

type MacOSMover struct{}

func (MacOSMover) MoveToTrash(ctx context.Context, path string) error {
	cmd := exec.CommandContext(
		ctx,
		"osascript",
		"-e", "on run argv",
		"-e", `tell application "Finder" to delete POSIX file (item 1 of argv)`,
		"-e", "end run",
		path,
	)
	return cmd.Run()
}
