package eval

import (
	"os"
	"testing"
)

func TestInfra(t *testing.T) {
	cases := []struct {
		r    Result
		want bool
	}{
		{Result{Findings: []Finding{{"run", "boxer run: cause: START_FAILED"}}}, true},
		{Result{Findings: []Finding{{"run", "npm ERR! code EIDLETIMEOUT"}}}, true},
		{Result{Findings: []Finding{{"run", "claude timed out"}}}, true},
		{Result{Findings: []Finding{{"run", "initialize: agent closed: EOF"}}}, true},
		{Result{Findings: []Finding{{"run", "session/prompt: agent closed: EOF"}}, Raw: `<- {"method":"session/update","params":{"update":{"sessionUpdate":"agent_message_chunk"}}}`}, false},
		{Result{Findings: []Finding{{"guest", `final answer "Darwin", wanted "Linux"`}}}, false},
		{Result{Status: "pass"}, false},
	}
	for _, c := range cases {
		if got := infra(c.r); got != c.want {
			t.Errorf("infra(%v) = %v, want %v", c.r.Findings, got, c.want)
		}
	}
}

func TestHostLockNestedIsNoop(t *testing.T) {
	t.Setenv("BOXER_EVAL_LOCKED", "1")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	unlock, err := HostLock(nil)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
	if _, err := os.Stat(LockPath()); err == nil {
		t.Fatal("nested HostLock must not touch the lock file")
	}
}
