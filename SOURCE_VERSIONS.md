# 源码版本与学习进度记录

记录时间：`2026-09-07T13:20:43+08:00`。用于重装系统后找到当前参考源码，并接着学习。

## 记录范围与读法

- 共整理本仓库的 16 个学习目录；在桌面找到 5 个对应源码仓库，另有 11 个未找到。
- 检索了 `/Users/bytedance/Desktop` 下的 Git 仓库，核对目录名和 `git remote -v`；跳过 `.git` 内部及依赖、虚拟环境、构建产物和工具缓存目录。只记录与本学习仓库对应的项目。
- 源码 commit 来自桌面源码仓库的 `HEAD`；手写进度 commit 来自 `handwritten-projects` 自身的提交历史，两者属于不同仓库。
- 已有学习版本优先保留并核实；没有旧版本记录的项目，使用本次桌面 HEAD 作为恢复参考，未逐文件确认手写代码与之完全一致。
- 全程使用本地 Git 信息，未执行 fetch、pull、checkout、commit 或 push。远程跟踪分支是本地缓存，不能据此确认服务器当前状态。
- 本文记录的是整理文档前的状态；本次 Markdown 改动不在下面的手写仓库 commit 中。重装前请将更新后的学习仓库推送或另行备份，确保这些记录也被保存。

## 已找到的源码版本

| 学习项目 | 源码仓库 | 本地分支 | 完整 commit | HEAD 对应 tag |
| --- | --- | --- | --- | --- |
| [Go/handwritten-conc](Go/handwritten-conc/readme.md) | [conc](https://github.com/sourcegraph/conc.git) | `main` | `5f936abd7ae87036af1f75c95fb9d0daaf00116b` | 无 |
| [Go/handwritten-timingwheel](Go/handwritten-timingwheel/readme.md) | [timingwheel](https://github.com/RussellLuo/timingwheel.git) | `master` | `54845bda31084d50d98d83afb01bd89d48154668` | 无 |
| [Go/handwritten_eino](Go/handwritten_eino/readme.md) | [eino](https://github.com/cloudwego/eino.git) | `main` | `a040cb96bbe1ea850b61d84cfb578f38c2d58cbc` | `v0.8.7` |
| [Rust/handwritten-claw-code](Rust/handwritten-claw-code/readme.md) | [claw-code](https://github.com/ultraworkers/claw-code.git) | `main` | `357629dbd9b300cbea7f484b6df263a862e20a84` | 无 |
| [python/handwritten_mem0](python/handwritten_mem0/README.md) | [mem0](https://github.com/mem0ai/mem0.git) | `main` | `a734e057cf8d6864318103f3f543fd02d330f09f` | 无 |

这 5 个项目的 README 均包含源码原路径、提交时间和可直接使用的 clone / 定位 commit 命令。

## 所有本地分支与跟踪状态

以下列出这 5 个源码仓库的所有本地分支；每个仓库均只有 1 个本地分支。差异以采集时本地缓存的远程跟踪引用为准。

| 源码目录（相对桌面） | 本地分支 | 分支 tip commit | 跟踪分支 | 跟踪分支缓存 commit | 差异 |
| --- | --- | --- | --- | --- | --- |
| `go_source/conc` | `main` | `5f936abd7ae87036af1f75c95fb9d0daaf00116b` | `origin/main` | `5f936abd7ae87036af1f75c95fb9d0daaf00116b` | 与缓存一致 |
| `go_source/timingwheel` | `master` | `54845bda31084d50d98d83afb01bd89d48154668` | `origin/master` | `54845bda31084d50d98d83afb01bd89d48154668` | 与缓存一致 |
| `ai-agent/eino` | `main` | `a040cb96bbe1ea850b61d84cfb578f38c2d58cbc` | `origin/main` | `ca0441ac0bceed8945dcf7d5a18c237c924c6aa8` | 落后 127 个提交 |
| `ai-agent/claw-code` | `main` | `357629dbd9b300cbea7f484b6df263a862e20a84` | `origin/main` | `357629dbd9b300cbea7f484b6df263a862e20a84` | 与缓存一致 |
| `ai-agent/mem0` | `main` | `a734e057cf8d6864318103f3f543fd02d330f09f` | `origin/main` | `a734e057cf8d6864318103f3f543fd02d330f09f` | 与缓存一致 |

特别注意：eino 的学习基准仍是 `v0.8.7` / `a040cb96bbe1ea850b61d84cfb578f38c2d58cbc`。直接使用以后克隆得到的 `main` 不能保证回到这个学习版本。

## 源码仓库未提交改动

| 源码仓库 | Git 状态 | 内容 |
| --- | --- | --- |
| conc | ` M stream/stream.go` | `callbackChPool` 声明中，`var` 后从 1 个空格变为 2 个空格。 |
| eino | ` M compose/tool_node.go` | `GetToolCallID` 中 `if !ok` 块的右花括号后新增行尾制表符。 |
| timingwheel / claw-code / mem0 | 干净 | 无已跟踪文件改动、无未跟踪文件。 |

这 5 个源码仓库均无 stash。上面两处改动均未暂存，`git diff --ignore-all-space HEAD` 为空；记录的 commit 不包含它们。这里只记录改动说明，没有导出工作区文件或补丁；被 Git 忽略的文件也未纳入记录。

## 桌面未找到的源码仓库

以下上游地址和版本线索来自原 README，没有可供核验的桌面 Git 源码仓库，所以不猜测分支或 commit。

| 学习项目 | 原 README 上游地址 | 原版本线索 | 确切 commit |
| --- | --- | --- | --- |
| [C/handwritten-ffmplay](C/handwritten-ffmplay/README.md) | [https://github.com/FFmpeg/FFmpeg](https://github.com/FFmpeg/FFmpeg.git) | 未记录 | 待补充 |
| [C/handwritten-redis](C/handwritten-redis/README.md) | [https://github.com/redis/redis](https://github.com/redis/redis.git) | 分支链接 `7.2`（未在本机核实） | 待补充 |
| [C++/handwritten-leveldb](C++/handwritten-leveldb/README.md) | [https://github.com/google/leveldb](https://github.com/google/leveldb.git) | 未记录 | 待补充 |
| [Go/handwritten-frp](Go/handwritten-frp/README.md) | [https://github.com/fatedier/frp](https://github.com/fatedier/frp.git) | 未记录 | 待补充 |
| [Java/handwritten-kafka](Java/handwritten-kafka/README.md) | [https://github.com/apache/kafka](https://github.com/apache/kafka.git) | 分支链接 `3.7`（未在本机核实） | 待补充 |
| [Rust/handwritten-chatgpt](Rust/handwritten-chatgpt/README.md) | [https://github.com/lencx/ChatGPT](https://github.com/lencx/ChatGPT.git) | 未记录 | 待补充 |
| [Rust/handwritten-rustdesk](Rust/handwritten-rustdesk/README.md) | [https://github.com/rustdesk/rustdesk](https://github.com/rustdesk/rustdesk.git) | 未记录 | 待补充 |
| [Rust/handwritten-tokio](Rust/handwritten-tokio/README.md) | [https://github.com/tokio-rs/tokio](https://github.com/tokio-rs/tokio.git) | 未记录 | 待补充 |
| [Rust/rust-practice/ctjhoa-rust-learning](Rust/rust-practice/ctjhoa-rust-learning/README.md) | [https://github.com/ctjhoa/rust-learning](https://github.com/ctjhoa/rust-learning.git) | 未记录 | 待补充 |
| [Rust/rust-practice/exercism-rust](Rust/rust-practice/exercism-rust/README.md) | [https://github.com/exercism/rust](https://github.com/exercism/rust.git) | 未记录 | 待补充 |
| [Rust/rust-practice/google-comprehensive-rust](Rust/rust-practice/google-comprehensive-rust/README.md) | [https://github.com/google/comprehensive-rust](https://github.com/google/comprehensive-rust.git) | 未记录 | 待补充 |

## 手写仓库恢复入口

```text
repo: git@github.com:Norxus/handwritten-projects.git
branch: master
commit_before_this_record: 8aa7a662d468ff706f18c4078349a2edff780a1b
commit_date: 2026-09-07T11:17:51+08:00
commit_subject: chore: daily learning
upstream: origin/master
upstream_cached_commit: 8aa7a662d468ff706f18c4078349a2edff780a1b
```

整理前仅有本地分支 `master`，与缓存的 `origin/master` 一致，工作区干净，无 stash。重装后克隆已保存了本次文档的学习仓库，日常学习继续使用保存后的最新版本。若要查看本次整理前的代码，可按下面的 commit 查看历史。

```bash
git clone git@github.com:Norxus/handwritten-projects.git handwritten-projects
git -C handwritten-projects show --stat 8aa7a662d468ff706f18c4078349a2edff780a1b
```

## 各目录最近一次手写提交

下表从本学习仓库的 Git 历史提取，截止于本次文档整理前；只表示最近一次修改记录，不据此推断学习是否完成。

| 学习目录 | 最近提交时间 | 完整 commit | 提交说明 |
| --- | --- | --- | --- |
| [C/handwritten-ffmplay](C/handwritten-ffmplay/README.md) | `2024-03-02T18:17:10+08:00` | `259efb237d22075a4f51ce7eb88d4b1ff3881266` | add ffmpeg |
| [C/handwritten-redis](C/handwritten-redis/README.md) | `2024-04-07T21:12:43+08:00` | `2729840572a1b79d477e29c574990509a079309a` | daily update 20240407 |
| [C++/handwritten-leveldb](C++/handwritten-leveldb/README.md) | `2024-04-07T21:12:43+08:00` | `2729840572a1b79d477e29c574990509a079309a` | daily update 20240407 |
| [Go/handwritten-conc](Go/handwritten-conc/readme.md) | `2026-05-11T13:14:56+08:00` | `f6c6316948b33c92ba5604254843560ee97810df` | feat: add eino, mem0, claw-code |
| [Go/handwritten-frp](Go/handwritten-frp/README.md) | `2024-04-07T21:12:43+08:00` | `2729840572a1b79d477e29c574990509a079309a` | daily update 20240407 |
| [Go/handwritten-timingwheel](Go/handwritten-timingwheel/readme.md) | `2026-05-11T13:14:56+08:00` | `f6c6316948b33c92ba5604254843560ee97810df` | feat: add eino, mem0, claw-code |
| [Go/handwritten_eino](Go/handwritten_eino/readme.md) | `2026-09-07T11:17:51+08:00` | `8aa7a662d468ff706f18c4078349a2edff780a1b` | chore: daily learning |
| [Java/handwritten-kafka](Java/handwritten-kafka/README.md) | `2024-04-07T21:12:43+08:00` | `2729840572a1b79d477e29c574990509a079309a` | daily update 20240407 |
| [Rust/handwritten-chatgpt](Rust/handwritten-chatgpt/README.md) | `2024-03-02T18:04:50+08:00` | `d05e89c6660aacacde5bb3d683a9ba397b568838` | add some projects |
| [Rust/handwritten-claw-code](Rust/handwritten-claw-code/readme.md) | `2026-08-14T16:23:44+08:00` | `65f710afc0f89c036b2ca43451de6264407c8829` | chore: daily learning and practice |
| [Rust/handwritten-rustdesk](Rust/handwritten-rustdesk/README.md) | `2024-03-02T18:04:50+08:00` | `d05e89c6660aacacde5bb3d683a9ba397b568838` | add some projects |
| [Rust/handwritten-tokio](Rust/handwritten-tokio/README.md) | `2024-03-02T18:04:50+08:00` | `d05e89c6660aacacde5bb3d683a9ba397b568838` | add some projects |
| [Rust/rust-practice/ctjhoa-rust-learning](Rust/rust-practice/ctjhoa-rust-learning/README.md) | `2024-03-02T18:04:50+08:00` | `d05e89c6660aacacde5bb3d683a9ba397b568838` | add some projects |
| [Rust/rust-practice/exercism-rust](Rust/rust-practice/exercism-rust/README.md) | `2024-03-02T18:04:50+08:00` | `d05e89c6660aacacde5bb3d683a9ba397b568838` | add some projects |
| [Rust/rust-practice/google-comprehensive-rust](Rust/rust-practice/google-comprehensive-rust/README.md) | `2024-04-07T21:12:43+08:00` | `2729840572a1b79d477e29c574990509a079309a` | daily update 20240407 |
| [python/handwritten_mem0](python/handwritten_mem0/README.md) | `2026-08-14T16:23:44+08:00` | `65f710afc0f89c036b2ca43451de6264407c8829` | chore: daily learning and practice |
| [`C/ffplay.c`](C/ffplay.c) | `2024-04-07T21:12:43+08:00` | `2729840572a1b79d477e29c574990509a079309a` | daily update 20240407 |

恢复后查看某项目最近一次修改及具体差异，例如：

```bash
git -C handwritten-projects show 8aa7a662d468ff706f18c4078349a2edff780a1b -- Go/handwritten_eino
git -C handwritten-projects show 65f710afc0f89c036b2ca43451de6264407c8829 -- Rust/handwritten-claw-code python/handwritten_mem0
```

## 本次使用的主要 Git 命令

在对应的源码仓库目录执行以下命令获取版本与本地状态：

```bash
git remote -v
git branch --show-current
git rev-parse HEAD
git show -s --format='%H%n%cI%n%s' HEAD
git tag --points-at HEAD
git for-each-ref --format='%(refname:short) %(objectname) %(upstream:short) %(upstream:track)' refs/heads
git for-each-ref --format='%(refname:short) %(objectname)' refs/remotes/origin/main refs/remotes/origin/master
git status --porcelain=v1 --untracked-files=all
git stash list
git diff --no-ext-diff --no-textconv --ignore-all-space HEAD
```

在手写仓库内，用 `git log -1 --format=fuller -- <学习目录>` 查看该目录最近提交。
