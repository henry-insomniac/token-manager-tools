package platform

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func ValidatePersistentExecutable(executable string) error {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return errors.New("无法确定当前可执行文件路径")
	}
	tempDir := strings.TrimSpace(os.TempDir())
	cleanExec := filepath.Clean(executable)
	if tempDir != "" {
		cleanTemp := filepath.Clean(tempDir)
		if cleanExec == cleanTemp || strings.HasPrefix(cleanExec, cleanTemp+string(os.PathSeparator)) {
			return errors.New("当前是临时可执行文件，无法保证重启后仍可自启；请改用 go build 后的二进制或发布包")
		}
	}
	return nil
}
