package box

import (
	"os"
	"path/filepath"
	"strings"
)

// Volumes are the one piece of guest state boxer keeps on purpose. A sandbox is disposable, and
// that is right for everything a worktree can rebuild; a database's files are not in that
// category, and losing them to idle reclaim or `--recreate` makes a sandbox useless for anything
// that migrates and seeds. So `volumes = ["pgdata:/var/lib/postgresql/data"]` keeps a directory on
// the host per scope, mounts it read-write, and removes it only when the worktree itself is gone
// or someone asks with `boxer rm --volumes`.

// VolumeDir is where the named volumes of every scope live on the host.
func VolumeDir() string { return filepath.Join(stateRoot(), "volumes") }

// rootFile records which worktree a scope's volumes belong to, so the sweep can tell when it is
// gone without asking a backend that may no longer have the sandbox.
const rootFile = ".worktree"

// volumeMounts creates the scope's volume directories and returns them as host:guest mounts.
func (e *Env) volumeMounts() ([]string, error) {
	if len(e.Cfg.Volumes) == 0 {
		return nil, nil
	}
	dir := filepath.Join(VolumeDir(), e.Scope.Key)
	var out []string
	for _, v := range e.Cfg.Volumes {
		name, guest, _ := strings.Cut(v, ":")
		host := filepath.Join(dir, name)
		if err := os.MkdirAll(host, 0o755); err != nil {
			return nil, err
		}
		out = append(out, host+":"+guest)
	}
	return out, os.WriteFile(filepath.Join(dir, rootFile), []byte(e.Scope.Root), 0o644)
}

// RemoveVolumes deletes every named volume of one scope.
func RemoveVolumes(key string) error {
	if key == "" || strings.ContainsAny(key, `/\`) {
		return nil
	}
	return os.RemoveAll(filepath.Join(VolumeDir(), key))
}

// OrphanVolumes lists the scopes whose volumes belong to a worktree that no longer exists. A
// scope without a record is left alone: boxer cannot prove whose it is, and a volume is exactly
// the thing not to guess about.
func OrphanVolumes() []string {
	ents, err := os.ReadDir(VolumeDir())
	if err != nil {
		return nil
	}
	var out []string
	for _, ent := range ents {
		b, err := os.ReadFile(filepath.Join(VolumeDir(), ent.Name(), rootFile))
		if err != nil || len(b) == 0 {
			continue
		}
		if _, err := os.Stat(string(b)); os.IsNotExist(err) {
			out = append(out, ent.Name())
		}
	}
	return out
}
