package fs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	tools "github.com/OctoSucker/octosucker-tools"
)

const providerName = "github.com/OctoSucker/tools-fs"

type SkillFs struct {
	mu    sync.RWMutex
	roots []string
}

func (s *SkillFs) Init(config map[string]interface{}, submitTask func(string) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if config == nil {
		s.roots = nil
		return nil
	}

	raw, _ := config["workspace_dirs"].([]interface{})
	roots, err := normalizeRoots(raw)
	if err != nil {
		return fmt.Errorf("skill-fs: %w", err)
	}
	s.roots = roots

	return nil
}

func (s *SkillFs) Cleanup() error {
	s.mu.Lock()
	s.roots = nil
	s.mu.Unlock()
	return nil
}

func (s *SkillFs) getAllowedRoots() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.roots))
	copy(out, s.roots)
	return out
}

func (s *SkillFs) Register(registry *tools.ToolRegistry, agent interface{}, providerName string) error {
	registry.RegisterTool(providerName, &tools.Tool{
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

	registry.RegisterTool(providerName, &tools.Tool{
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

	registry.RegisterTool(providerName, &tools.Tool{
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
	tools.RegisterToolProvider(&tools.ToolProviderInfo{
		Name:        providerName,
		Description: "文件系统 - 在工作区目录内读写、编辑文件",
		Provider:    globalSkillFs,
	})
}
