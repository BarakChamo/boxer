package main

import (
	"io"
	"os"
)

// readEarly reads its source from the moment it is called and hands the bytes on when somebody
// asks for them.
//
// A sandbox takes seconds to attach: the VM has to be resolved, started, and its environment made
// ready before the guest command exists. Until then nothing is reading this process's standard
// input, and the bytes sit in the terminal's own buffer — where they are discarded the moment the
// guest terminal is opened, because opening a terminal for reading flushes what is queued on it.
//
// That silently loses whatever a caller wrote first, and a caller that configures its shell
// immediately after starting it loses exactly the configuration it depends on. OpenHands writes a
// PROMPT_COMMAND that installs the prompt it parses command results out of, one second after it
// spawns the shell, and then waits forever for output that is delimited by a prompt that was never
// installed.
//
// The bytes go through an OS pipe rather than a Go buffer. os/exec copies a stdin that is not a
// file in a goroutine of its own and waits for that copy before Wait returns, and the copy was
// blocked reading a stdin that had not closed: a guest that exited left `boxer run --tty` waiting
// for the next keystroke, and on docker that keystroke turned exit 0 into a backend error. A pipe
// is a file, so the CLI reads it directly and Wait waits for nothing but the CLI.
// ponytail: a pipe holds 64 KB before the copy waits; typed input before the guest attaches is
// far less, and input from a pipe is not lost by waiting.
func readEarly(src io.Reader) io.Reader {
	r, w, err := os.Pipe()
	if err != nil {
		return src
	}
	go func() {
		_, _ = io.Copy(w, src)
		_ = w.Close()
	}()
	return r
}
