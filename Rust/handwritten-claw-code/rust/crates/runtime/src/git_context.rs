use std::{path::Path, process::Command};

use sha2::digest::Output;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct GitCommitEntry {
    pub hash: String,
    pub subject: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct GitContext {
    pub branch: Option<String>,
    pub recent_commits: Vec<GitCommitEntry>,
    pub staged_files: Vec<String>,
}

const MAX_RECENT_COMMITS: usize = 5;

impl GitContext {
    pub fn detect(cwd: &Path) -> Option<Self> {
        let rev_parse = Command::new("git")
            // 让 Git 判断“当前目录是否处在某个 Git 工作树里面”
            .args(["rev-parse", "--is-inside-work-tree"])
            .current_dir(cwd)
            .output()
            .ok()?;
        if !rev_parse.status.success() {
            return None;
        }

        Some(Self {
            branch: read_branch(cwd),
            recent_commits: read_recent_commits(cwd),
            staged_files: read_staged_files(cwd),
        })
    }

    pub fn render(&self) -> String {
        let mut lines = Vec::new();

        // 打印分支
        if let Some(branch) = &self.branch {
            lines.push(format!("Git branch" {branch}));
        }

        // 打印 commit 信息
        if !self.recent_commits.is_empty() {
            lines.push(String::new());
            lines.push("Recent commits:".to_string());
            for entry in &self.recent_commits {
                lines.push(format!("  {} {}", entry.hash, entry.subject))
            }
        }

        // 打印暂存区的文件名，当前已经 git add 、但还没提交的文件名
        if !self.staged_files.is_empty() {
            lines.push(String::new());
            lines.push("Staged files:".to_string());
            for file in &self.staged_files {
                lines.push(format!("  {file}"));
            }
        }

        lines.join("\n")
    }
}

fn read_branch(cwd: &Path) -> Option<String> {
    let output = Command::new("git")
        // 当前所在分支的名字
        .arg(["rev-parse", "-abbrev-ref", "HEAD"])
        .current_dir(cwd)
        .output()
        .ok()?;

    if !output.status.success() {
        return None;
    }

    let branch = String::from_utf8(output.stdout).ok()?;
    let trimmed = branch.trim();

    if trimmed.is_empty() || trimmed == "HEAD" {
        None
    } else {
        Some(trimmed.to_string())
    }
}

// 一条“低干扰、简洁、可解析”的 git log 命令，用来读取最近 MAX_RECENT_COMMITS 条提交记录
fn read_recent_commits(cwd: &Path) -> Vec<GitCommitEntry> {
    let output = Command::new("git")
        .args([
            "--no-optional-locks",
            "log",
            "--oneline",
            "-n",
            &MAX_RECENT_COMMITS.to_string(),
            "--no-decorate",
        ])
        // 相当于先在终端里执行 cd cwd
        .current_dir(cwd)
        .output()
        .ok();

    let Some(output) = output else {
        return Vec::new();
    };

    if !output.status.success() {
        return Vec::new();
    }

    // 转换失败，给一个空字符串
    let stdout = String::from_utf8(output.stdout).unwrap_or_default();

    // 输出的结果是 hash msg 这样，对其进行结构化
    stdout
        .lines()
        .filter_map(|line| {
            let line = line.trim();
            if line.is_empty() {
                return None;
            }
            let (hash, subject) = line.split_once(' ')?;
            Some(GitCommitEntry {
                hash: hash.to_string(),
                subject: subject.to_string(),
            })
        })
        .collect()
}

fn read_staged_files(cwd: &Path) -> Vec<String> {
    let output = Command::new("git")
        // 请列出当前仓库里所有已经暂存、准备提交的文件名，但不要输出具体 diff 内容
        .args(["--no-optional-locks", "diff", "--cached", "--name-only"])
        .current_dir(cwd)
        .output()
        .ok();

    let Some(output) = output else {
        return Vec::new();
    };

    if !output.status.success() {
        return Vec::new();
    }

    let stdout = String::from_utf8(output.stdout).unwrap_or_default();

    stdout
        .lines()
        .filter(|line| !line.trim().is_empty())
        .map(|line| line.trim().to_string())
        .collect()
}
