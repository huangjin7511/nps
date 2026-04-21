package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStatusUsesTrimmedPIDFile(t *testing.T) {
	oldProcessExists := processExistsFn
	t.Cleanup(func() {
		processExistsFn = oldProcessExists
	})

	var gotPID int
	processExistsFn = func(pid int) bool {
		gotPID = pid
		return true
	}

	pidDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pidDir, "nps.pid"), []byte(" 123 \n"), 0600); err != nil {
		t.Fatalf("WriteFile(pid) error = %v", err)
	}

	if !status("nps", pidDir) {
		t.Fatal("status() = false, want true")
	}
	if gotPID != 123 {
		t.Fatalf("status() used pid %d, want 123", gotPID)
	}
}

func TestStatusRejectsInvalidPIDFile(t *testing.T) {
	oldProcessExists := processExistsFn
	t.Cleanup(func() {
		processExistsFn = oldProcessExists
	})

	called := false
	processExistsFn = func(pid int) bool {
		called = true
		return true
	}

	pidDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pidDir, "nps.pid"), []byte("12oops"), 0600); err != nil {
		t.Fatalf("WriteFile(pid) error = %v", err)
	}

	if status("nps", pidDir) {
		t.Fatal("status() = true, want false for invalid pid file")
	}
	if called {
		t.Fatal("status() called processExists for invalid pid file")
	}
}

func TestStopUsesParsedPID(t *testing.T) {
	oldProcessExists := processExistsFn
	oldTerminateProcess := terminateProcessFn
	t.Cleanup(func() {
		processExistsFn = oldProcessExists
		terminateProcessFn = oldTerminateProcess
	})

	processExistsFn = func(pid int) bool {
		return pid == 456
	}

	var gotPID int
	terminateProcessFn = func(pid int) error {
		gotPID = pid
		return nil
	}

	pidDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pidDir, "nps.pid"), []byte("456"), 0600); err != nil {
		t.Fatalf("WriteFile(pid) error = %v", err)
	}

	stop("nps", "unused", pidDir)
	if gotPID != 456 {
		t.Fatalf("stop() used pid %d, want 456", gotPID)
	}
}

func TestStartSetsWorkingDirectory(t *testing.T) {
	pidDir := t.TempDir()
	runPath := t.TempDir()
	outputDir := t.TempDir()
	outputFile := filepath.Join(outputDir, "cwd.txt")

	t.Setenv("GO_WANT_DAEMON_HELPER", "1")
	t.Setenv("DAEMON_HELPER_OUT", outputFile)

	start([]string{os.Args[0], "-test.run=TestDaemonHelperProcess", "--"}, "daemon-helper", pidDir, runPath)

	deadline := time.Now().Add(5 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("helper did not write %s before timeout", outputFile)
		}
		data, err := os.ReadFile(outputFile)
		if err == nil {
			workingDir := strings.TrimSpace(string(data))
			if workingDir == "" {
				time.Sleep(25 * time.Millisecond)
				continue
			}
			if workingDir != runPath {
				t.Fatalf("helper working directory = %q, want %q", workingDir, runPath)
			}
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func TestStartInvalidCommandDoesNotPanic(t *testing.T) {
	pidDir := t.TempDir()
	missing := filepath.Join(t.TempDir(), "missing-daemon-command")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("start() panicked: %v", r)
		}
	}()

	start([]string{missing}, "daemon-missing", pidDir, "")

	if _, err := os.Stat(filepath.Join(pidDir, "daemon-missing.pid")); !os.IsNotExist(err) {
		t.Fatalf("pid file stat error = %v, want not-exist", err)
	}
}

func TestDaemonHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_DAEMON_HELPER") != "1" {
		return
	}

	outputFile := os.Getenv("DAEMON_HELPER_OUT")
	if outputFile == "" {
		os.Exit(2)
	}

	wd, err := os.Getwd()
	if err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(outputFile, []byte(wd), 0600); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
