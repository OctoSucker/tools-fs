package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// isUnderRoot 判断 canonicalPath 是否在 canonicalRoot 之下（含等于）。
// 要求 canonicalPath 已通过 EvalSymlinks 得到，无符号链接逃逸。
func isUnderRoot(canonicalPath, canonicalRoot string) bool {
	rel, err := filepath.Rel(canonicalRoot, canonicalPath)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

// resolveAndValidate 将 path 解析为绝对路径并检查一定落在任一允许的工作区根目录下。
// - 相对路径（如 "test.txt"）视为相对于第一个 workspace_dir。
// - 绝对路径必须在某 workspace_dir 之下。
// - 禁止通过 ".." 逃逸；且对已存在路径做真实路径（EvalSymlinks）检查，防止通过符号链接访问工作区外。
// roots 应为已规范化的绝对路径（建议为 canonical，见 normalizeRoots）。
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

	// 1) 字符串层面：必须在某 root 下，禁止 ".." 逃逸
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

	// 2) 真实路径检查：防止通过符号链接访问或创建到工作区外
	canonicalAbs, err := filepath.EvalSymlinks(abs)
	if err == nil {
		// 路径已存在：真实路径必须在任一 canonical root 下
		for _, root := range roots {
			if isUnderRoot(canonicalAbs, root) {
				return abs, nil
			}
		}
		return "", fmt.Errorf("path %q resolves outside workspace_dirs (symlink escape)", path)
	}
	// 路径不存在（新文件）：若父目录已存在，其真实路径必须在工作区内，否则可能通过 symlink 写到外面
	parent := filepath.Dir(abs)
	if canonicalParent, errParent := filepath.EvalSymlinks(parent); errParent == nil {
		for _, root := range roots {
			if isUnderRoot(canonicalParent, root) {
				return abs, nil
			}
		}
		return "", fmt.Errorf("path %q parent resolves outside workspace_dirs (symlink escape)", path)
	}
	// 父目录不存在：由 MkdirAll 在工作区内按路径创建，字符串已通过 1) 校验
	return abs, nil
}

// expandTilde 将路径开头的 ~ 展开为当前用户 HOME 目录（配置中常用 "~/workspace"）。
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

// normalizeRoots 将配置中的根目录列表转为规范化绝对路径（真实路径，解析符号链接）；支持 "~"。
// 若某条根目录不存在则自动创建，确保后续校验一致。
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
