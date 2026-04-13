package directorypicker

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

type MacOSPicker struct{}

func (MacOSPicker) PickDirectory(ctx context.Context) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", errors.New("directory picker only supported on macOS")
	}

	cmd := exec.CommandContext(ctx, "osascript", "-e", `POSIX path of (choose folder with prompt "选择影栈存储目录")`)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}
