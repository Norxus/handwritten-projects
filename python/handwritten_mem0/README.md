# handwritten_mem0

用于抄写与学习 `mem0` 记忆系统核心实现的空项目。

## 目录约定

- `mem0/`: 逐步抄写的核心包代码
- `.venv/`: 本地虚拟环境

## 快速开始

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -U pip
pip install -e ".[dev,nlp]"
```

## 源码版本记录（2026-09-07）

```text
repo: https://github.com/mem0ai/mem0.git
branch: main
tag: 无指向 HEAD 的本地 tag
commit: a734e057cf8d6864318103f3f543fd02d330f09f
commit_date: 2026-05-05T13:48:21-07:00
source_path: /Users/bytedance/Desktop/ai-agent/mem0
recorded_at: 2026-09-07T13:20:43+08:00
```

提交说明：fix (telemetry): stitch oss and platform telemetry identities for python and typescript sdk

源码工作区：干净（无已跟踪文件改动或未跟踪文件；忽略文件未计入）；stash：无。

以上是桌面源码仓库的当前版本，作为重装后的源码参考；未逐文件确认手写代码与该版本完全对应。

### 恢复源码

在新系统的源码父目录执行：

```bash
git clone https://github.com/mem0ai/mem0.git mem0
git -C mem0 switch --detach a734e057cf8d6864318103f3f543fd02d330f09f
```

完整索引、所有本地分支和手写进度见[源码版本与学习进度记录](../../SOURCE_VERSIONS.md)。
