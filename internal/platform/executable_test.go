package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePersistentExecutable(t *testing.T) {
	if err := ValidatePersistentExecutable("/Applications/Token Manager Tools/token-manager"); err != nil {
		t.Fatalf("expected persistent executable to pass: %v", err)
	}

	tempExec := filepath.Join(os.TempDir(), "go-build-token-manager", "token-manager")
	err := ValidatePersistentExecutable(tempExec)
	if err == nil {
		t.Fatalf("expected temp executable to be rejected")
	}
	if !strings.Contains(err.Error(), "临时可执行文件") {
		t.Fatalf("unexpected error: %v", err)
	}
}
