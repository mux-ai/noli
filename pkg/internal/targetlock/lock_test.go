package targetlock

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAcquireSerializesAndCanBeReacquired(t *testing.T) {
	path := filepath.Join(t.TempDir(), "write.lock")
	first, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()

	if _, err := Acquire(path); !errors.Is(err, ErrBusy) {
		t.Fatalf("second Acquire() error = %v, want ErrBusy", err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	third, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire() after release = %v", err)
	}
	if err := third.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestAcquireSerializesAcrossProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "write.lock")
	command := exec.Command(os.Args[0], "-test.run=^TestLockHelperProcess$")
	command.Env = append(os.Environ(), "NOLI_LOCK_TEST_PATH="+path)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	if err != nil || line != "locked\n" {
		t.Fatalf("helper readiness = %q, error = %v", line, err)
	}

	if _, err := Acquire(path); !errors.Is(err, ErrBusy) {
		t.Fatalf("cross-process Acquire() error = %v, want ErrBusy", err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}

	lock, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire() after helper exit = %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestLockHelperProcess(t *testing.T) {
	path := os.Getenv("NOLI_LOCK_TEST_PATH")
	if path == "" {
		return
	}
	lock, err := Acquire(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Fprintln(os.Stdout, "locked")
	_, _ = io.Copy(io.Discard, os.Stdin)
	if err := lock.Release(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	os.Exit(0)
}
