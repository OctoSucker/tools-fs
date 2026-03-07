# skill-fs

文件系统 Skill，为 OctoSucker Agent 提供在工作区目录内**读、写、编辑**文件的能力。

## 工具

| 工具 | 说明 |
|------|------|
| `read_file` | 读取指定路径文件内容。路径必须在 `workspace_dirs` 之下。 |
| `write_file` | 将内容写入指定路径（覆盖）。路径必须在 `workspace_dirs` 之下。 |
| `edit_file` | 在文件中将 `old_content` 的首次出现替换为 `new_content`。 |

## 配置

在 Agent 配置文件（如 `config/agent_config.json`）中增加 `fs` 段：

```json
{
  "llm": { ... },
  "fs": {
    "workspace_dirs": [".", "/path/to/allowed/root"]
  }
}
```

- **workspace_dirs**（必填）：允许访问的根目录列表。可为相对路径（相对进程当前工作目录）或绝对路径。
- 所有 `read_file` / `write_file` / `edit_file` 的路径会被解析为绝对路径后，检查是否落在任一 `workspace_dirs` 之下；禁止通过 `..` 逃逸到目录外。

## 安全（操作一定在工作区内）

- **保证**：所有读/写/编辑操作一定发生在配置的 `workspace_dirs` 之下，不会访问或创建工作区外文件。
- 未配置 `workspace_dirs` 或列表为空时，工具会返回明确错误，不进行任何文件访问。
- **路径校验**：
  1. 字符串层面：路径必须落在某条 `workspace_dir` 下，禁止通过 `..` 逃逸。
  2. 真实路径层面：对已存在路径与父目录做 `filepath.EvalSymlinks`，确保符号链接不会指向工作区外；若解析结果在工作区外则直接拒绝。
- `workspace_dirs` 在 Init 时会被规范为真实路径（解析符号链接），若目录不存在会先创建再规范。

## 安装

在 OctoSucker 主模块中：

```bash
go get github.com/OctoSucker/skill-fs@latest
```

并保证 `main.go` 中有空白导入以触发注册：

```go
import _ "github.com/OctoSucker/skill-fs"
```

本地开发时在 `go.mod` 中使用 `replace` 指向本仓库即可。
