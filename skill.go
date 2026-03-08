package fs

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	skill "github.com/OctoSucker/octosucker-skill"
)

const skillName = "github.com/OctoSucker/skill-fs"

// SkillFs 文件系统 Skill
type SkillFs struct {
	mu    sync.RWMutex
	roots []string // 允许的根目录（绝对路径）
}

// Init 初始化 Skill，从 config 读取 workspace_dirs
func (s *SkillFs) Init(config map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if config == nil {
		log.Printf("skill-fs: no config, file access disabled until workspace_dirs is set")
		s.roots = nil
		return nil
	}

	raw, _ := config["workspace_dirs"].([]interface{})
	roots, err := normalizeRoots(raw)
	if err != nil {
		return fmt.Errorf("skill-fs: %w", err)
	}
	s.roots = roots
	if len(roots) > 0 {
		log.Printf("skill-fs: workspace_dirs=%v", roots)
	} else {
		log.Printf("skill-fs: workspace_dirs empty, file access disabled")
	}
	return nil
}

// Cleanup 清理 Skill
func (s *SkillFs) Cleanup() error {
	s.mu.Lock()
	s.roots = nil
	s.mu.Unlock()
	return nil
}

func (s *SkillFs) getAllowedRoots() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.roots) == 0 {
		return nil
	}
	out := make([]string, len(s.roots))
	copy(out, s.roots)
	return out
}

// RegisterFsSkill 向 registry 注册 read_file / write_file / edit_file
func RegisterFsSkill(registry *skill.ToolRegistry, agent interface{}) error {
	registry.Register(&skill.Tool{
		Name:        "read_file",
		Description: "读取指定路径的文件内容。相对路径（如 test.txt）会在第一个工作区目录下解析；绝对路径必须在 workspace_dirs 内。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "文件路径。相对路径（如 test.txt、subdir/file.txt）会在工作区内解析；绝对路径必须在 workspace_dirs 内。",
				},
			},
			"required": []string{"path"},
		},
		Handler: handleReadFile,
	})

	registry.Register(&skill.Tool{
		Name:        "write_file",
		Description: "将内容写入指定路径。相对路径（如 test.txt）会在第一个工作区目录下创建；绝对路径必须在 workspace_dirs 内。文件不存在会创建，存在则覆盖。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "文件路径。相对路径（如 test.txt、subdir/file.txt）会在工作区内创建；绝对路径必须在 workspace_dirs 内。",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "要写入的完整内容",
				},
			},
			"required": []string{"path", "content"},
		},
		Handler: handleWriteFile,
	})

	registry.Register(&skill.Tool{
		Name:        "edit_file",
		Description: "在文件中将 old_content 的首次出现替换为 new_content。相对路径在工作区内解析；绝对路径必须在 workspace_dirs 内。若未找到 old_content 则返回错误。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "文件路径。相对路径在工作区内解析；绝对路径必须在 workspace_dirs 内。",
				},
				"old_content": map[string]interface{}{
					"type":        "string",
					"description": "要被替换的原文（首次匹配）",
				},
				"new_content": map[string]interface{}{
					"type":        "string",
					"description": "替换后的新内容",
				},
			},
			"required": []string{"path", "old_content", "new_content"},
		},
		Handler: handleEditFile,
	})

	return nil
}

func handleReadFile(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	path, ok := params["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("path is required")
	}
	roots := globalSkillFs.getAllowedRoots()
	if len(roots) == 0 {
		return nil, fmt.Errorf("skill-fs: workspace_dirs not configured. Add workspace_dirs to skill-fs config to allow file access")
	}
	resolved, err := resolveAndValidate(path, roots)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return map[string]interface{}{
		"success": true,
		"path":    resolved,
		"content": string(data),
	}, nil
}

func handleWriteFile(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	path, ok := params["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("path is required")
	}
	content, ok := params["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content is required")
	}
	roots := globalSkillFs.getAllowedRoots()
	if len(roots) == 0 {
		return nil, fmt.Errorf("skill-fs: workspace_dirs not configured. Add workspace_dirs to skill-fs config to allow file access")
	}
	resolved, err := resolveAndValidate(path, roots)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create parent dir: %w", err)
	}
	if err := os.WriteFile(resolved, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}
	return map[string]interface{}{
		"success": true,
		"path":    resolved,
	}, nil
}

func handleEditFile(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	path, ok := params["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("path is required")
	}
	oldContent, ok := params["old_content"].(string)
	if !ok {
		return nil, fmt.Errorf("old_content is required")
	}
	newContent, ok := params["new_content"].(string)
	if !ok {
		return nil, fmt.Errorf("new_content is required")
	}
	roots := globalSkillFs.getAllowedRoots()
	if len(roots) == 0 {
		return nil, fmt.Errorf("skill-fs: workspace_dirs not configured. Add workspace_dirs to skill-fs config to allow file access")
	}
	resolved, err := resolveAndValidate(path, roots)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("read file for edit: %w", err)
	}
	s := string(data)
	if !strings.Contains(s, oldContent) {
		return nil, fmt.Errorf("old_content not found in file")
	}
	// 只替换第一次出现
	out := strings.Replace(s, oldContent, newContent, 1)
	if err := os.WriteFile(resolved, []byte(out), 0644); err != nil {
		return nil, fmt.Errorf("write file after edit: %w", err)
	}
	return map[string]interface{}{
		"success": true,
		"path":    resolved,
	}, nil
}

var globalSkillFs *SkillFs

func init() {
	globalSkillFs = &SkillFs{}
	skill.RegisterSkillWithMetadata(
		skillName,
		skill.SkillMetadata{
			Name:        skillName,
			Version:     "0.1.0",
			Description: "文件系统 Skill - 在工作区目录内读写、编辑文件",
			Author:      "OctoSucker",
			Tags:        []string{"filesystem", "file", "read", "write", "edit"},
		},
		RegisterFsSkill,
		globalSkillFs,
	)
}
