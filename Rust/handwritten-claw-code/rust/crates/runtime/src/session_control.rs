use std::{
    fmt::{format, Display, Formatter},
    fs,
    path::{self, Path, PathBuf},
    time::{self, SystemTime, UNIX_EPOCH},
};

use crate::{json::JsonError, session::Session, ConversationMessage};

pub const PRIMARY_SESSION_EXTENSION: &str = "jsonl";
pub const LEGACY_SESSION_EXTENSION: &str = "json";
pub const LATEST_SESSION_REFERENCE: &str = "latest";
const SESSION_REFERENCE_ALIASES: &[&str] = &[LATEST_SESSION_REFERENCE, "last", "recent"];

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SessionStore {
    sessions_root: PathBuf,
    workspace_root: PathBuf,
}

pub struct ManagedSessionSummary {
    pub id: String,
    pub path: PathBuf,
    pub update_at_ms: u64,
    pub modified_epoch_millis: u128,
    pub message_count: usize,
    pub parent_session_id: Option<String>,
    pub branch_name: Option<String>,
}

impl SessionStore {
    // 根据目录哈希，并在 ./claw/sessions 目录下创建一个目录
    // AsRef 是一个“轻量借用转换” trait，表示“这个类型可以被看成另一个类型的引用”
    // 意思是： from_cwd 接受任何“能借用成 &Path ”的参数
    // 也就是通过借用能变成 &Path 的类型， 比如传 &Path 、 PathBuf 、 &str 、 String 等路径风格的值（它们都实现了 AsRef<Path>）
    pub fn from_cwd(cwd: impl AsRef<Path>) -> Result<Self, SessionControlError> {
        let cwd = cwd.as_ref();

        // 归一化，相对路径转换为绝对路径，如果规一化失败就退回使用原路径
        let canonical_cwd = fs::canonicalize(cwd).unwrap_or_else(|_| cwd.to_path_buf());
        // 为了把 session 文件按“当前工作区”隔离存放，避免不同项目互相冲突
        // 创建一个“当前工作区专属的 session 命名空间”
        let sessions_root = canonical_cwd
            .join(".claw")
            .join("sessions")
            .join(workspace_fingerprint(&canonical_cwd));
        fs::create_dir_all(&sessions_root)?;
        Ok(Self {
            sessions_root,
            workspace_root: canonical_cwd,
        })
    }

    // 往上走一层，返回父 sessions 目录
    fn legacy_sessions_root(&self) -> Option<PathBuf> {
        self.sessions_root
            .parent()
            .filter(|parent| parent.file_name().is_some_and(|name| name == "sessions"))
            .map(Path::to_path_buf)
    }

    // 校验“从磁盘加载出来的 session”是不是属于当前 SessionStore 绑定的工作区
    fn validate_loaded_session(
        &self,
        session_path: &Path,
        session: &Session,
    ) -> Result<(), SessionControlError> {
        let Some(actual) = session.workspace_root() else {
            // 如果 session 的 workspace_root 是否存在，不存在那么就当作 “旧版 legacy session”
            if path_is_within_workspace(session_path, &self.workspace_root) {
                return Ok(());
            }
            // 如果不在当前工作区内，就报错
            return Err(SessionControlError::Format(
                format_legacy_session_missing_workspace_root(session_path, &self.workspace_root),
            ));
        };
        // 如果 session 中有记录 root，那么比较一下 sessionStore 记录的 root
        // 不匹配就报错
        if workspace_roots_match(actual, &self.workspace_root) {
            return Ok(());
        }
        Err(SessionControlError::WorkspaceMismatch {
            expected: self.workspace_root.clone(),
            actual: actual.to_path_buf(),
        })
    }

    pub fn list_sessions(&self) -> Result<Vec<ManagedSessionSummary>, SessionControlError> {
        let mut sessions = Vec::new();
        self.collect_sessions_from_dir(&self.sessions_root, &mut sessions)?;
        if let Some(legacy_root) = self.legacy_sessions_root() {
            self.collect_sessions_from_dir(&legacy_root, &mut sessions)?;
        }
        sort_managed_sessions(&mut sessions);
        Ok(sessions)
    }

    // 扫描某个目录的会话文件，把能识别出的会话整理成 ManagedSessionSummary
    fn collect_sessions_from_dir(
        &self,
        directory: &Path,
        sessions: &mut Vec<ManagedSessionSummary>,
    ) -> Result<(), SessionControlError> {
        let entries = match fs::read_dir(directory) {
            Ok(entries) => entries,
            // 如果不存在，那么就不需要追加，直接退出
            Err(err) if err.kind() == std::io::ErrorKind::NotFound => return Ok(()),
            Err(err) => return Err(err.into()),
        };
        for entry in entries {
            let entry = entry?;
            let path = entry.path();
            // 必须要符合格式
            if !is_managed_session_file(&path) {
                continue;
            }
            let metadata = entry.metadata()?;
            let modified_epoch_millis = metadata
                .modified()
                .ok()
                // UNIX_EPOCH 就是 1970-01-01 00:00:00 UTC
                // 把 SystemTime 转成“距离 Unix epoch 的时长”
                .and_then(|time| time.duration_since(UNIX_EPOCH).ok())
                // 把时长转成毫秒数
                .map(|duration| duration.as_millis())
                .unwrap_or_default();
            let summary = match Session::load_from_path(&path) {
                Ok(session) => {
                    if self.validate_loaded_session(&path, &session).is_err() {
                        continue;
                    }
                    ManagedSessionSummary {
                        id: session.session_id,
                        path,
                        update_at_ms: session.update_at_ms,
                        modified_epoch_millis,
                        message_count: session.messages.len(),
                        parent_session_id: session
                            .fork
                            .as_ref()
                            .map(|fork| fork.parent_session_id.clone()),
                        branch_name: session
                            .fork
                            .as_ref()
                            .and_then(|fork| fork.branch_name.clone()),
                    }
                }
                Err(_) => ManagedSessionSummary {
                    id: path
                        .file_stem()
                        .and_then(|value| value.to_str())
                        .unwrap_or("unknown")
                        .to_string(),
                    path,
                    update_at_ms: 0,
                    modified_epoch_millis,
                    message_count: 0,
                    parent_session_id: None,
                    branch_name: None,
                },
            };
            sessions.push(summary);
        }
        Ok(())
    }

    // 遍历获取最近的 session
    pub fn latest_session(&self) -> Result<ManagedSessionSummary, SessionControlError> {
        self.list_sessions()?.into_iter().next().ok_or_else(|| {
            SessionControlError::Format(format_no_managed_sessions(&self.sessions_root))
        })
    }

    pub fn resolve_reference(&self, reference: &str) -> Result<SessionHandle, SessionControlError> {
        // 如果传入的是 "latest" 、 "last" 、 "recent" 之一 （特殊的指代），
        // 就取当前工作区下最新的会话并返回它的句柄
        if is_session_reference_alias(reference) {
            let latest = self.latest_session()?;
            return Ok(SessionHandle {
                id: latest.id,
                path: latest.path,
            });
        }

        let direct = PathBuf::from(reference);
        // 获取绝对路径
        let candidate = if direct.is_absolute() {
            direct.clone()
        } else {
            self.workspace_root.join(&direct)
        };

        // 有文件扩展名 或者 有目录层级/路径结构，就把它判定为“看起来像路径”
        let looks_like_path = direct.extension().is_some() || direct.components().count() > 1;
        let path = if candidate.exists() {
            candidate
        } else if looks_like_path {
            // 如果这个路径不存在，但它“看起来像路径”，那就直接报错
            return Err(SessionControlError::Format(
                format_missing_session_reference(reference, &self.sessions_root),
            ));
        } else {
            // 如果它既不存在，又“不像路径”，那就认为它更可能是一个 session ID
            // 使用 session_id 获取文件路径
            self.resolve_managed_path(reference)?
        };

        Ok(SessionHandle {
            id: session_id_from_path(&path).unwrap_or_else(|| reference.to_string()),
            path,
        })
    }

    // 根据 session_id 找出对应的 session 文件，返回文件路径
    pub fn resolve_managed_path(&self, session_id: &str) -> Result<PathBuf, SessionControlError> {
        for extension in [PRIMARY_SESSION_EXTENSION, LEGACY_SESSION_EXTENSION] {
            let path = self.sessions_root.join(format!("{session_id}.{extension}"));
            if path.exists() {
                return Ok(path);
            }
        }
        // 如果当前目录没找到，再去 legacy 目录里查找同样两个文件名
        if let Some(legacy_root) = self.legacy_sessions_root() {
            for extension in [PRIMARY_SESSION_EXTENSION, LEGACY_SESSION_EXTENSION] {
                let path = legacy_root.join(format!("{session_id}.{extension}"));
                if !path.exists() {
                    continue;
                }
                // 如果在 legacy 目录里找到了，还会把文件加载出来做一次校验，确认这个 session 确实属于当前 workspace
                let session = Session::load_from_path(&path)?;
                self.validate_loaded_session(&path, &session)?;
                return Ok(path);
            }
        }
        Err(SessionControlError::Format(
            format_missing_session_reference(session_id, &self.sessions_root),
        ))
    }

    pub fn load_session(
        &self,
        reference: &str,
    ) -> Result<LoadedManagedSession, SessionControlError> {
        let handle = self.resolve_reference(reference)?;
        let session = Session::load_from_path(&handle.path)?;
        self.validate_loaded_session(&handle.path, &session)?;
        Ok(LoadedManagedSession {
            handle: SessionHandle {
                id: session.session_id.clone(),
                path: handle.path,
            },
            session,
        })
    }
}

fn format_missing_session_reference(reference: &str, sessions_root: &Path) -> String {
    let fingerprint_dir = sessions_root
        .file_name()
        .and_then(|f| f.to_str())
        .unwrap_or("<unknown>");
    format!(
        "session not found: {reference}\nHint: managed sessions live in .claw/sessions/{fingerprint_dir}/ (workspace-specific partition).\nTry `{LATEST_SESSION_REFERENCE}` for the most recent session or `/session list` in the REPL."
    )
}

// 从 session 文件夹路径里提取 session_id
fn session_id_from_path(path: &Path) -> Option<String> {
    path.file_name()
        .and_then(|value| value.to_str())
        .and_then(|name| {
            name.strip_suffix(&format!(".{PRIMARY_SESSION_EXTENSION}"))
                .or_else(|| name.strip_suffix(&format!(".{LEGACY_SESSION_EXTENSION}")))
        })
        .map(ToOwned::to_owned)
}

// 按照 updated_at_ms、modified_epoch、id 依次降序排
fn sort_managed_sessions(sessions: &mut [ManagedSessionSummary]) {
    sessions.sort_by(|left, right| {
        right
            .update_at_ms
            .cmp(&left.update_at_ms)
            // 如果前一次比较已经分出大小，就直接用那个结果，如果相等，才继续看下一条规则
            .then_with(|| right.modified_epoch_millis.cmp(&left.modified_epoch_millis))
            .then_with(|| right.id.cmp(&left.id))
    });
}

// 规范化后进行相等比较
fn workspace_roots_match(left: &Path, right: &Path) -> bool {
    canonicalize_for_compare(left) == canonicalize_for_compare(right)
}

// 检测 path 是否在 workspace_root 之下
fn path_is_within_workspace(path: &Path, workspace_root: &Path) -> bool {
    canonicalize_for_compare(path).starts_with(canonicalize_for_compare(workspace_root))
}

// 规范化路径
fn canonicalize_for_compare(path: &Path) -> PathBuf {
    fs::canonicalize(path).unwrap_or_else(|_| path.to_path_buf())
}

// 生成错误提示信息，这个旧版 session 缺少 workspace 绑定信息
fn format_legacy_session_missing_workspace_root(
    session_path: &Path,
    workspace_root: &Path,
) -> String {
    format!(
        "legacy session is missing workspace binding: {}\nOpen it from its original workspace or re-save it from {}.",
        session_path.display(),
        workspace_root.display()
    )
}

// 生成一段“没有找到受管 session”时的错误提示文案
fn format_no_managed_sessions(sessions_root: &Path) -> String {
    // 获取 sessions_root 最后的一段目录名，获取失败就返回 <unknown>
    let fingerprint_dir = sessions_root
        .file_name()
        .and_then(|f| f.to_str())
        .unwrap_or("<unknown>");
    format!(
        "no managed sessions found in .claw/sessions/{fingerprint_dir}/\nStart `claw` to create a session, then rerun with `--resume {LATEST_SESSION_REFERENCE}`.\nNote: claw partitions sessions per workspace fingerprint; sessions from other CWDs are invisible."
    )
}

// 判断一个路径是不是“受管理的 session 文件”（格式是否是 json 和 jsonl）
#[must_use]
pub fn is_managed_session_file(path: &Path) -> bool {
    path.extension()
        .and_then(|ext| ext.to_str())
        .is_some_and(|extension| {
            extension == PRIMARY_SESSION_EXTENSION || extension == LEGACY_SESSION_EXTENSION
        })
}

pub fn is_session_reference_alias(reference: &str) -> bool {
    SESSION_REFERENCE_ALIASES
        .iter()
        .any(|alias| reference.eq_ignore_ascii_case(alias))
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LoadedManagedSession {
    pub handle: SessionHandle,
    pub session: Session,
}

pub struct SessionHandle {
    pub id: String,
    pub path: PathBuf,
}

// 直接拿完整路径当目录名不够理想：可能太长、包含特殊字符、不同平台表现也不统一
// 保证“同一个目录的不同写法得到同一个指纹”
pub fn workspace_fingerprint(workspace_root: &Path) -> String {
    let input = workspace_root.to_string_lossy();
    let mut hash = 0xcbf2_9ce4_8422_2325_u64;
    // 对这个字符串的每个字节做一次 64 位 FNV-1a 哈希
    for byte in input.as_bytes() {
        hash ^= u64::from(*byte);
        hash = hash.wrapping_mul(0x0100_0000_01b3);
    }
    format!("{hash:016x}")
}

pub enum SessionControlError {
    Io(std::io::Error),
    Session(SessionError),
    Format(String),
    WorkspaceMismatch { expected: PathBuf, actual: PathBuf },
}

impl Display for SessionControlError {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Io(error) => write!(f, "{error}"),
            Self::Session(error) => write!(f, "{error}"),
            Self::Format(error) => write!(f, "{error}"),
            Self::WorkspaceMismatch { expected, actual } => write!(
                f,
                "session workspace mismatch: expected {}, found {}",
                expected.display(),
                actual.display(),
            ),
        }
    }
}

impl std::error::Error for SessionControlError {}

pub enum SessionError {
    Io(std::io::Error),
    Json(JsonError),
    Format(String),
}

impl Display for SessionError {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Io(error) => write!(f, "{error}"),
            Self::Json(error) => write!(f, "{error}"),
            Self::Format(error) => write!(f, "{error}"),
        }
    }
}

impl std::error::Error for SessionError {}
