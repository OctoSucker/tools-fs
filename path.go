package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func isUnderRoot(canonicalPath, canonicalRoot string) bool {
	rel, err := filepath.Rel(canonicalRoot, canonicalPath)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

func resolveAndValidate(path string, roots []string) (string, error) {
	if len(roots) == 0 {
		return "", fmt.Errorf("skill-fs: no workspace_dirs configured, file access is disabled")
	}

	path = filepath.Clean(path)
	var abs string
	if filepath.IsAbs(path) {
		var err error
		abs, err = filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("invalid path %q: %w", path, err)
		}
		abs = filepath.Clean(abs)
	} else {
		abs = filepath.Clean(filepath.Join(roots[0], path))
	}

	var underRoot string
	for _, root := range roots {
		rootAbs := filepath.Clean(root)
		rel, err := filepath.Rel(rootAbs, abs)
		if err != nil {
			continue
		}
		if rel == "." || !strings.HasPrefix(rel, "..") {
			underRoot = rootAbs
			break
		}
	}
	if underRoot == "" {
		return "", fmt.Errorf("path %q is outside allowed workspace_dirs", path)
	}

	canonicalAbs, err := filepath.EvalSymlinks(abs)
	if err == nil {
		for _, root := range roots {
			if isUnderRoot(canonicalAbs, root) {
				return abs, nil
			}
		}
		return "", fmt.Errorf("path %q resolves outside workspace_dirs (symlink escape)", path)
	}
	parent := filepath.Dir(abs)
	if canonicalParent, errParent := filepath.EvalSymlinks(parent); errParent == nil {
		for _, root := range roots {
			if isUnderRoot(canonicalParent, root) {
				return abs, nil
			}
		}
		return "", fmt.Errorf("path %q parent resolves outside workspace_dirs (symlink escape)", path)
	}
	return abs, nil
}

func expandTilde(s string) string {
	s = strings.TrimSpace(s)
	if s == "~" || strings.HasPrefix(s, "~/") || strings.HasPrefix(s, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return s
		}
		return home + s[1:]
	}
	return s
}

func normalizeRoots(raw []interface{}) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(raw))
	seen := make(map[string]bool)
	for _, v := range raw {
		s, ok := v.(string)
		if !ok {
			continue
		}
		s = expandTilde(s)
		if s == "" {
			continue
		}
		abs, err := filepath.Abs(s)
		if err != nil {
			return nil, fmt.Errorf("invalid workspace_dir %q: %w", s, err)
		}
		abs = filepath.Clean(abs)
		if err := os.MkdirAll(abs, 0755); err != nil {
			return nil, fmt.Errorf("create workspace_dir %q: %w", abs, err)
		}
		canonical, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return nil, fmt.Errorf("resolve workspace_dir %q: %w", abs, err)
		}
		canonical = filepath.Clean(canonical)
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		out = append(out, canonical)
	}
	return out, nil
}
