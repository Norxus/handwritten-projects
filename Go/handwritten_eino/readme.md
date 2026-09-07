# handwritten_eino

## 源码版本记录（2026-09-07）

```text
repo: https://github.com/cloudwego/eino.git
branch: main
tag: v0.8.7
commit: a040cb96bbe1ea850b61d84cfb578f38c2d58cbc
commit_date: 2026-04-07T12:11:44+08:00
source_path: /Users/bytedance/Desktop/ai-agent/eino
recorded_at: 2026-09-07T13:20:43+08:00
```

提交说明：feat(skill): add extension hooks for fork skill execution and tool params (#905)

源码工作区：`compose/tool_node.go` 有未暂存修改；`git diff --ignore-all-space HEAD` 为空，仅有空白字符变化。stash：无。

已通过 Git 核实：原 README 的 `v0.8.7` 和 commit 均与桌面源码 HEAD 一致。当前分支虽为 `main`，但比本地缓存的 `origin/main` 落后 127 个提交；恢复时按上述 commit 定位。

### 恢复源码

在新系统的源码父目录执行：

```bash
git clone https://github.com/cloudwego/eino.git eino
git -C eino switch --detach a040cb96bbe1ea850b61d84cfb578f38c2d58cbc
```

完整索引、所有本地分支和手写进度见[源码版本与学习进度记录](../../SOURCE_VERSIONS.md)。
