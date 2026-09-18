package box

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/BarakChamo/boxer/internal/vm"
)

// Footprint is what boxer costs this host in bytes, and what can be reclaimed. Storage is the
// question users actually ask — a sandbox is cheap to make and easy to forget — so boxer reports
// it rather than leaving `du` to answer.
type Footprint struct {
	Machines  int   `json:"machines"`   // sandboxes smolvm holds for boxer
	PackCount int   `json:"pack_count"` // cached images and harness installs
	PackBytes int64 `json:"pack_bytes"`
	FreeBytes int64 `json:"free_bytes"` // on the filesystem holding the packs
}

// Usage measures the pack cache and the free space around it. A missing pack directory is not an
// error: it means nothing has been cached yet.
func Usage(machines int) Footprint {
	f := Footprint{Machines: machines}
	packs, _ := filepath.Glob(filepath.Join(PackDir(), "*.smolmachine"))
	for _, p := range packs {
		if st, err := os.Stat(p); err == nil {
			f.PackCount++
			f.PackBytes += st.Size()
		}
	}
	f.FreeBytes = FreeBytes(PackDir())
	return f
}

// FreeBytes is the space available to this user on the filesystem holding dir, or its nearest
// existing parent. It returns 0 when the question cannot be answered, and every caller treats 0
// as "unknown" rather than as "full", because refusing to work on a bad statfs would be worse
// than the problem it guards against.
func FreeBytes(dir string) int64 {
	for d := dir; d != "" && d != "/"; d = filepath.Dir(d) {
		var st syscall.Statfs_t
		if err := syscall.Statfs(d, &st); err == nil {
			return int64(st.Bavail) * int64(st.Bsize)
		}
		if _, err := os.Stat(d); err == nil {
			return 0 // the directory exists and statfs still failed: unknown
		}
	}
	return 0
}

// packingWouldCrowdTheDisk reports whether writing a pack of about this size would leave the host
// with less than the configured margin. A pack is a cache: skipping one costs a slow image pull,
// while filling the disk breaks every VM on the machine, boxer's and everyone else's. This host
// filled up twice while boxer was being written, both times mid-pack.
func packingWouldCrowdTheDisk(minFree int64) (int64, bool) {
	if minFree <= 0 {
		return 0, false
	}
	free := FreeBytes(PackDir())
	if free == 0 {
		return 0, false // unknown: proceed, the write itself will fail honestly
	}
	return free, free < minFree
}

// StaleScratch lists eval scratch directories older than idle. The evaluation suite keeps failed
// cells for inspection and each one is a repository and a rendered package; on a machine that
// runs the tiers often they are the largest thing boxer leaves behind.
func StaleScratch(dir string, idle time.Duration) []string {
	if idle <= 0 {
		return nil
	}
	entries, _ := filepath.Glob(filepath.Join(dir, "boxer-eval-*"))
	var out []string
	for _, e := range entries {
		st, err := os.Stat(e)
		if err != nil || !st.IsDir() || time.Since(st.ModTime()) <= idle {
			continue
		}
		out = append(out, e)
	}
	return out
}

// ReclaimDue reports whether an automatic sweep is due, and records that one is being started.
// The stamp is a file, because boxer has no daemon and no state beyond smolvm's labels: whichever
// process asks first wins, the rest see a fresh stamp and skip. A sweep that crashes leaves the
// stamp, so the worst case is one skipped interval rather than a sweep loop.
func ReclaimDue(every time.Duration) bool {
	if every <= 0 {
		return false
	}
	stamp := filepath.Join(filepath.Dir(LastUsedDir()), "last-reclaim")
	if st, err := os.Stat(stamp); err == nil && time.Since(st.ModTime()) < every {
		return false
	}
	if err := os.MkdirAll(filepath.Dir(stamp), 0o755); err != nil {
		return false
	}
	// Create-or-touch before sweeping: two boxers starting together must not both sweep.
	f, err := os.OpenFile(stamp, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false
	}
	_ = f.Close()
	now := time.Now()
	_ = os.Chtimes(stamp, now, now)
	return true
}

// Resources is what one sandbox costs: what smolvm allocated, what its process is actually using,
// and what it holds on disk. Allocation alone is misleading — a VM given 4 GB may be using 200 MB —
// and disk is the part that surprises people, because it grows quietly.
type Resources struct {
	CPUs        int     `json:"cpus"`                   // allocated
	MemoryMiB   int     `json:"memory_mib"`             // allocated
	PID         int     `json:"pid,omitempty"`          // the machine's process on the host
	RSSMiB      int     `json:"rss_mib,omitempty"`      // actually resident
	CPUPercent  float64 `json:"cpu_percent,omitempty"`  // as the host sees it
	DiskBytes   int64   `json:"disk_bytes,omitempty"`   // the machine's data directory
	MeasuredAll bool    `json:"measured_all,omitempty"` // false when something could not be read
}

// Usage of one machine. A stopped machine has no process, so only its disk is measurable, and the
// caller gets zeros rather than an error: a resource report that fails is worse than one that is
// partial and says so.
func MachineResources(m vm.Machine, dataDir string) Resources {
	r := Resources{CPUs: m.CPUs, MemoryMiB: m.MemoryMiB, PID: m.PID, MeasuredAll: true}
	if m.PID > 0 {
		if rss, cpu, ok := processUsage(m.PID); ok {
			r.RSSMiB, r.CPUPercent = rss, cpu
		} else {
			r.MeasuredAll = false
		}
	}
	if dataDir != "" {
		if n, ok := dirSize(dataDir); ok {
			r.DiskBytes = n
		} else {
			r.MeasuredAll = false
		}
	}
	return r
}

// processUsage reads resident memory and CPU share from the host's own process table. `ps` is the
// portable answer here: /proc does not exist on macOS and a cgo dependency for two numbers would
// cost more than it returns.
func processUsage(pid int) (rssMiB int, cpuPercent float64, ok bool) {
	out, err := exec.Command("ps", "-o", "rss=,%cpu=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, 0, false
	}
	f := strings.Fields(string(out))
	if len(f) != 2 {
		return 0, 0, false
	}
	kb, err1 := strconv.Atoi(f[0])
	cpu, err2 := strconv.ParseFloat(f[1], 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return kb / 1024, cpu, true
}

// dirSize sums what a directory tree actually occupies, in allocated blocks rather than apparent
// size. A machine's disks are sparse files: a VM given a 20 GB disk and using 300 MB reports 20 GB
// by length and 300 MB by blocks, and only the second is a number anyone can act on. This is what
// `du` reports and why it disagrees with `ls -l`.
func dirSize(dir string) (int64, bool) {
	var total int64
	ok := true
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			ok = false
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if st, isUnix := info.Sys().(*syscall.Stat_t); isUnix {
			total += st.Blocks * 512
			return nil
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return total, false
	}
	return total, ok
}
