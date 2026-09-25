package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// SiteOrigin is where the docs site is published. The discovery index states absolute URLs,
// because a client that found the domain has nothing else to resolve against.
const SiteOrigin = "https://boxer.sh"

// SkillName is the one skill boxer publishes. The Agent Skills format requires the declared name
// to match the directory holding SKILL.md, which is why this is a constant rather than a path.
const SkillName = "boxer"

// RenderSkills writes the skill twice, from the same template the plugin is rendered from:
//
//   - skills/<name>/SKILL.md, the non-hidden layout that skill installers (`gh skill install`,
//     `npx skills add`) read. They should not have to reach into a repo-local .agents projection.
//   - a .well-known/agent-skills index beside the published site, with a SHA-256 digest of each
//     file, so a client that knows only the domain can find and verify the skill without a
//     GitHub-specific registry.
//
// Both are generated, never hand-edited, and CI diffs them: two copies of a document that drift
// are worse than one copy in the wrong place.
func RenderSkills(version, skillsDir, wellKnownDir string) ([]string, error) {
	pkg, err := renderPackage(version)
	if err != nil {
		return nil, err
	}
	const prefix = "skills/" + SkillName + "/"
	skill := map[string][]byte{}
	for rel, body := range pkg {
		if after, ok := strings.CutPrefix(rel, prefix); ok {
			skill[after] = body
		}
	}
	body, ok := skill["SKILL.md"]
	if !ok {
		return nil, fmt.Errorf("the package carries no %sSKILL.md", prefix)
	}
	var written []string
	if skillsDir != "" {
		files, err := writeAll(filepath.Join(skillsDir, SkillName), skill)
		if err != nil {
			return nil, err
		}
		written = append(written, files...)
	}
	if wellKnownDir == "" {
		return written, nil
	}
	sum := sha256.Sum256(body)
	index, err := json.MarshalIndent(map[string]any{
		"schemaVersion": "0.2.0",
		"skills": []map[string]any{{
			"name":        SkillName,
			"description": skillDescription(body),
			"url":         SiteOrigin + "/.well-known/agent-skills/" + SkillName + "/SKILL.md",
			"digest":      "sha256:" + hex.EncodeToString(sum[:]),
			"version":     version,
		}},
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	files, err := writeAll(filepath.Join(wellKnownDir, "agent-skills"), map[string][]byte{
		SkillName + "/SKILL.md": body,
		"index.json":            append(index, '\n'),
	})
	return append(written, files...), err
}

// skillDescription reads the description out of the skill's own frontmatter, so the index cannot
// disagree with the document it points at.
func skillDescription(body []byte) string {
	for _, line := range strings.Split(string(body), "\n") {
		if rest, ok := strings.CutPrefix(line, "description:"); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}
