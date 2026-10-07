package main

import (
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// The point of readEarly is that input written before anybody reads it is kept rather than lost.
// A sandbox takes seconds to attach, and a caller that configures its shell immediately after
// starting it writes into that gap.
func TestInputWrittenBeforeAnybodyReadsItIsKept(t *testing.T) {
	pr, pw := io.Pipe()
	early := readEarly(pr)

	// Written now, read much later: exactly the shape of the bug this exists for.
	go func() {
		_, _ = io.WriteString(pw, "export PROMPT_COMMAND=x\n")
		time.Sleep(50 * time.Millisecond)
		_, _ = io.WriteString(pw, "echo hello\n")
		_ = pw.Close()
	}()
	time.Sleep(150 * time.Millisecond) // nothing is reading yet

	got, err := io.ReadAll(early)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := "export PROMPT_COMMAND=x\necho hello\n"
	if string(got) != want {
		t.Errorf("early input was lost: got %q, want %q", got, want)
	}
}

func TestReadEarlyEndsWhenItsSourceDoes(t *testing.T) {
	early := readEarly(strings.NewReader("one shot"))
	got, err := io.ReadAll(early)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "one shot" {
		t.Errorf("got %q", got)
	}
	// A second read past the end reports the end rather than blocking.
	n, err := early.Read(make([]byte, 4))
	if n != 0 || err == nil {
		t.Errorf("reading past the end gave n=%d err=%v", n, err)
	}
}

// A command given the early reader as stdin finishes when it exits, whether or not stdin ever
// closes: os/exec waited for its stdin copy, which was blocked on a stdin that stayed open.
func TestACommandEndsThoughItsEarlyStdinStaysOpen(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()
	cmd := exec.Command("sh", "-c", "exit 3")
	cmd.Stdin = readEarly(pr)
	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()
	select {
	case err := <-done:
		if cmd.ProcessState.ExitCode() != 3 {
			t.Fatalf("exit %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the command exited, but Wait is still waiting on stdin")
	}
}
