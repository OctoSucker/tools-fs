# tools-fs

文件系统 Tool Provider，为 OctoSucker Agent 提供在工作区目录内**读、写、编辑**文件的能力。

## 工具

| 工具 | 说明 |
|------|------|
| `read_file` | 读取指定路径文件内容。参数：`path`（必填）。相对路径（如 `test.txt`、`subdir/file.txt`）在**第一个**工作区目录下解析；绝对路径必须在 `workspace_dirs` 内。 |
| `write_file` | 将内容写入指定路径（不存在则创建，存在则覆盖）。参数：`path`、`content`（必填）。路径解析规则同 `read_file`。 |
| `edit_file` | 在文件中将 `old_content` 的**首次**出现替换为 `new_content`。参数：`path`、`old_content`、`new_content`（必填）。若未找到 `old_content` 则返回错误。路径解析规则同 `read_file`。 |

未配置 `workspace_dirs` 或列表为空时，三个工具均返回明确错误，不进行任何文件访问。

## 配置

在 Agent 配置的 `tool_providers["github.com/OctoSucker/tools-fs"]` 下：

| 键 | 说明 |
|------|------|
| `workspace_dirs` | 允许访问的根目录列表（必填）。可为相对路径（相对进程当前工作目录）或绝对路径。 |

示例（`config/agent_config.json`）：

```json
"github.com/OctoSucker/tools-fs": {
  "workspace_dirs": [".", "workspace", "/path/to/allowed/root"]
}
```

- 所有路径会被解析为绝对路径后，检查是否落在任一 `workspace_dirs` 之下；禁止通过 `..` 逃逸到目录外。
- `workspace_dirs` 在 Init 时会被规范为真实路径（解析符号链接）；若某条目录不存在会先创建再规范。

## 安全

- **路径校验**：路径必须落在某条 `workspace_dir` 下，禁止通过 `..` 逃逸；对已存在路径与父目录做 `filepath.EvalSymlinks`，确保符号链接不会指向工作区外。
- 未配置或空 `workspace_dirs` 时，不进行任何文件操作。

## 安装

主项目中：

```bash
go get github.com/OctoSucker/tools-fs@latest
```

并保留空白导入：`_ "github.com/OctoSucker/tools-fs"`。
