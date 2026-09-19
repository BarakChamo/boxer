package main

import (
	"bytes"
	"io"
	"sync"
)

// earlyReader reads its source from the moment it is created and hands the bytes on when somebody
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
type earlyReader struct {
	mu   sync.Mutex
	cond *sync.Cond
	buf  bytes.Buffer
	err  error
	done bool
}

// readEarly starts consuming src at once. The returned reader replays everything captured so far,
// then continues with whatever arrives.
func readEarly(src io.Reader) io.Reader {
	e := &earlyReader{}
	e.cond = sync.NewCond(&e.mu)
	go func() {
		chunk := make([]byte, 4096)
		for {
			n, err := src.Read(chunk)
			e.mu.Lock()
			if n > 0 {
				e.buf.Write(chunk[:n])
			}
			if err != nil {
				e.err, e.done = err, true
			}
			e.cond.Broadcast()
			e.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	return e
}

func (e *earlyReader) Read(p []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for e.buf.Len() == 0 && !e.done {
		e.cond.Wait()
	}
	if e.buf.Len() > 0 {
		return e.buf.Read(p)
	}
	return 0, e.err
}
