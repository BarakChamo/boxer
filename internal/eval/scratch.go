package eval

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ScratchPrefixes are the temp-directory prefixes the evaluation tiers create. Kept failures are
// deliberate — a failed cell's repository is the evidence — but "kept" had come to mean "kept
// forever": the sdlc tier never removed its base repository at all, and 96 of them held 66 GB of
// node_modules under $TMPDIR, which macOS prunes only after three days of no access.
var ScratchPrefixes = []string{"boxer-eval-", "boxer-sdlc-base-", "bxm-"}

// scratchMaxAge is how long a kept failure is worth keeping. Long enough to read the evidence of
// last night's run in the morning, short enough that a week of runs cannot fill a disk.
const scratchMaxAge = 3 * 24 * time.Hour

// PruneScratch removes eval scratch directories older than scratchMaxAge, and returns how many.
// It runs when an eval starts, so the suite bounds its own footprint rather than relying on
// someone remembering `make clean-evals`.
func PruneScratch(now time.Time) int {
	root := os.TempDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() || !hasScratchPrefix(e.Name()) || strings.HasSuffix(e.Name(), "-packs") {
			continue // the pack cache is shared across runs and reclaimed by `boxer gc`
		}
		info, err := e.Info()
		if err != nil || now.Sub(info.ModTime()) < scratchMaxAge {
			continue
		}
		if os.RemoveAll(filepath.Join(root, e.Name())) == nil {
			n++
		}
	}
	return n
}

func hasScratchPrefix(name string) bool {
	for _, p := range ScratchPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// bases are the scaffolded repositories this process created, for AbortCleanup.
var (
	basesMu sync.Mutex
	bases   []string
)

func trackBase(dir string) {
	basesMu.Lock()
	bases = append(bases, dir)
	basesMu.Unlock()
}

// AbortCleanup brings down every sandbox under a base repository this process created. It is for
// an interrupted run: each lifecycle downs its own sandbox in a deferred call, and a signal skips
// every one of them, so a stopped matrix left its sandboxes running against worktrees that still
// existed — which `gc` rightly leaves alone.
func AbortCleanup(boxerBin string) {
	basesMu.Lock()
	defer basesMu.Unlock()
	for _, base := range bases {
		dirs, _ := filepath.Glob(filepath.Join(base+"-worktrees", "*"))
		for _, d := range append(dirs, base) {
			_, _ = runIn(d, boxerBin, "down")
		}
		// An interrupted run proves nothing, so unlike a failed one it keeps no evidence.
		_ = os.RemoveAll(base + "-worktrees")
		_ = os.RemoveAll(base)
	}
}
