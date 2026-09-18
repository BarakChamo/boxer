package eval

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Drivers lists every harness and orchestrator driver in report order.
func Drivers() []Driver {
	return []Driver{
		Flow{},
		Claude{},
		Codex{},
		Gemini{},
		OpenCode{},
		Pi{},
		Kimi{},
		DSH{},
		Grok{},
		Copilot{},
		Inside{},
		InsideACP{},
		OpenHands{},
		&Paperclip{},
		&T3{},
		&Herdr{},
		Multica,
	}
}

// harnessTimeout bounds one harness invocation. A cell that has not answered in four minutes is a
// failure worth reporting, not something to wait out: the whole suite is sixty-odd cells. It is a
// variable so a test can prove the kill path without waiting four minutes for it.
var harnessTimeout = 4 * time.Minute

// wait runs cmd to completion, killing it after harnessTimeout. Every driver used to carry its
// own copy of this start/wait/kill dance, and they had drifted: some killed only the parent, some
// reported the timeout without the output that explains it. The name is the harness, so the
// timeout error reads as the driver's own.
func wait(cmd *exec.Cmd, name string) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(harnessTimeout):
		_ = cmd.Process.Kill()
		// Reap, but never wait forever for it: Wait blocks until every inherited pipe is closed,
		// and a harness that spawns children hands them the same pipe, so killing the parent does
		// not close it. A timed-out cell must end the cell.
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
		return fmt.Errorf("%s timed out after %s", name, harnessTimeout)
	}
}

// lastAnswer returns the first word of the last non-empty line of raw that starts with none of
// the given prefixes. Several harnesses print their answer as the last interesting line of a
// noisy stream, and each had written its own loop for it; what differs between them is only the
// noise to skip. The first word is what the oracle compares: the answers are kernel names.
func lastAnswer(raw string, skip ...string) string {
	answer := ""
	for _, line := range strings.Split(stripANSI(raw), "\n") {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		skipped := false
		for _, p := range skip {
			if strings.HasPrefix(l, p) {
				skipped = true
				break
			}
		}
		if f := strings.Fields(l); !skipped && len(f) > 0 {
			answer = f[0]
		}
	}
	return answer
}
