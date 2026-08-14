use std::{
    collections::BTreeMap,
    fmt::format,
    fs,
    path::{Path, PathBuf},
    sync::atomic::{AtomicU64, Ordering},
    time::{SystemTime, UNIX_EPOCH},
    u64,
};

use serde_json::value;

use crate::{
    json::JsonValue,
    session,
    session_control::{SessionError, LATEST_SESSION_REFERENCE},
};

const SESSION_VERSION: u32 = 1;
const ROTATE_AFTER_BYTES: u64 = 256 * 1024;
const MAX_ROTATED_FILES: usize = 3;
static SESSION_ID_COUNTER: AtomicU64 = AtomicU64::new(0);
static LAST_TIMESTAMP_MS: AtomicU64 = AtomicU64::new(0);

#[derive(Debug, Clone)]
pub struct Session {
    pub version: u32,
    pub session_id: String,
    pub created_at_ms: u64,
    pub updated_at_ms: u64,
    pub messages: Vec<ConversationMessage>,
    pub compaction: Option<SessionCompaction>,
    pub fork: Option<SessionFork>,
    pub workspace_root: Option<PathBuf>,
    pub prompt_history: Vec<SessionPromptEntry>,
    pub last_health_check_ms: Option<u64>,
    pub model: Option<String>,
    persistence: Option<SessionPersistence>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SessionCompaction {
    pub count: u32,
    pub removed_message_count: usize,
    pub summary: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SessionFork {
    pub parent_session_id: String,
    pub branch_name: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SessionPromptEntry {
    pub timestamp_ms: u64,
    pub text: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct SessionPersistence {
    path: PathBuf,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ConversationMessage {
    pub role: MessageRole,
    pub blocks: Vec<ContentBlock>,
    pub usage: Option<TokenUsage>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum MessageRole {
    System,
    User,
    Assistant,
    Tool,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ContentBlock {
    Text {
        text: String,
    },
    Thinking {
        thinking: String,
        signature: Option<String>,
    },
    ToolUse {
        id: String,
        name: String,
        input: String,
    },
    ToolResult {
        tool_use_id: String,
        tool_name: String,
        output: String,
        is_error: bool,
    },
}

#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct TokenUsage {
    pub input_tokens: u32,
    pub output_tokens: u32,
    pub cache_creation_input_tokens: u32,
    pub cache_read_input_tokens: u32,
}

impl Session {
    #[must_use]
    pub fn new() -> Self {
        let now = current_time_millis();
        Self {
            version: SESSION_VERSION,
            session_id: generate_session_id(),
            created_at_ms: now,
            updated_at_ms: now,
            messages: Vec::new(),
            compaction: None,
            fork: None,
            workspace_root: None,
            prompt_history: Vec::new(),
            last_health_check_ms: None,
            model: None,
            persistence: None,
        }
    }

    #[must_use]
    pub fn with_workspace_root(mut self, workspace_root: impl Into<PathBuf>) -> Self {
        self.workspace_root = Some(workspace_root.into());
        self
    }

    // 从路径文件下读取所有内容
    pub fn load_from_path(path: impl AsRef<Path>) -> Result<Self, SessionError> {
        let path = path.as_ref();
        let contents = fs::read_to_string(path)?;
        let session = match JsonValue::parse(&contents) {
            Ok(value)
                if value
                    .as_object()
                    // 这个 Option 必须是 Some(...)，并且里面的值还要满足一个条件
                    .is_some_and(|object| object.contains_key("messages")) =>
            {
                Self::from_json(&value)?
            }
            Err(_) | Ok(_) => Self::from_jsonl(&contents)?,
        };
        Ok(session.with_persistence_path(path.to_path_buf()))
    }

    fn touch(&mut self) {
        self.updated_at_ms = current_time_millis()
    }

    pub fn save_to_path(&self, path: impl AsRef<Path>) -> Result<(), SessionError> {
        let path = path.as_ref();
        // 把自己的一些属性信息总结
        let snapshot = self.render_jsonl_snapshot()?;
        // 检查旧文件是否过大，过大的话就把旧文件轮转归档
        rotate_session_file_if_needed(path)?;
        // 把总结写入文件
        write_atomic(path, &snapshot)?;
        // 清楚文件父目录下的历史归档文件
        cleanup_rotated_logs(path)?;
        Ok(())
    }

    fn render_jsonl_snapshot(&self) -> Result<String, SessionError> {
        // 先记录 Session 元信息
        let mut lines = vec![self.meta_record()?.render()];
        // 记录压缩信息
        if let Some(compaction) = &self.compaction {
            lines.push(compaction.to_jsonl_record()?.render());
        }
        // 把 prompt_history 的每一项都转成一行 JSON
        lines.extend(
            self.prompt_history
                .iter()
                .map(|entry| entry.to_jsonl_record().render()),
        );
        // 把 messages 里的每条消息都转成一行 JSON
        lines.extend(
            self.messages
                .iter()
                .map(|message| message_record(message).render()),
        );
        let mut rendered = lines.join("\n");
        rendered.push('\n');
        Ok(rendered)
    }

    // 把当前 Session 的“元信息”组装成一条 JSON 记录
    fn meta_record(&self) -> Result<JsonValue, SessionError> {
        let mut object = BTreeMap::new();
        object.insert(
            "type".to_string(),
            JsonValue::String("session_meta".to_string()),
        );
        object.insert(
            "version".to_string(),
            JsonValue::Number(i64::from(self.version)),
        );
        object.insert(
            "session_id".to_string(),
            JsonValue::String(self.session_id.clone()),
        );
        object.insert(
            "created_at_ms".to_string(),
            JsonValue::Number(i64_from_u64(self.created_at_ms, "created_at_ms")?),
        );
        object.insert(
            "updated_at_ms".to_string(),
            JsonValue::Number(i64_from_u64(self.updated_at_ms, "updated_at_ms")?),
        );
        if let Some(fork) = &self.fork {
            object.insert("fork".to_string(), fork.to_json());
        }
        if let Some(workspace_root) = &self.workspace_root {
            object.insert(
                "workspace_root".to_string(),
                JsonValue::String(workspace_root_to_string(workspace_root)?),
            );
        }
        if let Some(model) = &self.model {
            object.insert("model".to_string(), JsonValue::String(model.clone()));
        }
        Ok(JsonValue::Object(object))
    }

    // 在压缩完成后，把一次压缩的摘要元数据记到 session 上
    pub fn record_compaction(&mut self, summary: impl Into<String>, removed_message_count: usize) {
        // 更新会话的 updated_at_ms 时间戳
        self.touch();
        // 压缩次数 + 1
        let count = self.compaction.as_ref().map_or(1, |value| value.count + 1);
        // 维护 用所次数、压缩删除了多少消息、压缩生成的摘要文本
        self.compaction = Some(SessionCompaction {
            count,
            removed_message_count,
            summary: summary.into,
        });
    }

    // 从 JsonValue 中逐字段拼凑出 Session
    pub fn from_json(value: &JsonValue) -> Result<Self, SessionError> {
        let object = value
            .as_object()
            // Option<T> 转成 Result<T, E> 的方法
            // 如果前面的值是 Some(x) ，就变成 Ok(x)
            // 如果前面的值是 None ，就调用闭包 || ... 生成一个错误，变成 Err(...)
            .ok_or_else(|| SessionError::Format("session must be an object".to_string()))?;
        let version = object
            .get("version")
            // 调用 JsonValue::as_i64 函数
            .and_then(JsonValue::as_i64)
            .ok_or_else(|| SessionError::Format("session must be an object".to_string()))?;

        let version = u32::try_from(version)
            // 如果前面结果是 Err(original_error) ，就把错误映射成你指定的新错误
            .map_err(|_| SessionError::Format("version out of range".to_string()))?;
        let messages = object
            .get("messages")
            // 还原成数组
            .and_then(JsonValue::as_array)
            .ok_or_else(|| SessionError::Format("missing messages".to_string()))?
            .iter()
            .map(ConversationMessage::from_json)
            .collect::<Result<Vec<_>, _>>()?;
        let now = current_time_millis();
        let session_id = object
            .get("session_id")
            .and_then(JsonValue::as_str)
            .map_or_else(generate_session_id, ToOwned::to_owned);
        let created_at_ms = object
            .get("created_at_ms")
            .map(|value| required_u64_from_value(value, "created_at_ms"))
            .transpose()?
            .unwrap_or(now);
        let updated_at_ms = object
            .get("updated_at_ms")
            .map(|value| required_u64_from_value(value, "updated_at_ms"))
            .transpose()?
            .unwrap_or(created_at_ms);
        let compaction = object
            .get("compaction")
            .map(SessionCompaction::from_json)
            .transpose()?;
        let fork = object.get("fork").map(SessionFork::from_json).transpose()?;
        let workspace_root = object
            .get("workspace_root")
            .and_then(JsonValue::as_str)
            .map(PathBuf::from);
        let prompt_history = object
            .get("prompt_history")
            .and_then(JsonValue::as_array)
            .map(|entries| {
                entries
                    .iter()
                    .filter_map(SessionPromptEntry::from_json_opt)
                    .collect()
            })
            .unwrap_or_default();
        let model = object
            .get("model")
            .and_then(JsonValue::as_str)
            .map(String::from);
        Ok(Self {
            version,
            session_id,
            created_at_ms,
            updated_at_ms,
            messages,
            compaction,
            fork,
            workspace_root,
            prompt_history,
            last_health_check_ms: None,
            model,
            persistence: None,
        })
    }

    // 从文件中拼凑出 Self
    fn from_jsonl(contents: &str) -> Result<Self, SessionError> {
        let mut version = SESSION_VERSION;
        let mut session_id = None;
        let mut created_at_ms = None;
        let mut updated_at_ms = None;
        let mut messages = Vec::new();
        let mut compaction = None;
        let mut fork = None;
        let mut workspace_root = None;
        let mut model = None;
        let mut prompt_history = Vec::new();

        for (line_number, raw_line) in contents.lines().enumerate() {
            let line = raw_line.trim();
            if line.is_empty() {
                continue;
            }
            let value = JsonValue::parse(line).map_err(|error| {
                SessionError::Format(format!(
                    "invalid JSONL record at line {}: {}",
                    line_number + 1,
                    error
                ))
            })?;
            let object = value.as_object().ok_or_else(|| {
                SessionError::Format(format!(
                    "JSONL record at line {} must be an object",
                    line_number + 1
                ))
            })?;

            match object
                .get("type")
                .and_then(JsonValue::as_str)
                .ok_or_else(|| {
                    SessionError::Format(format!(
                        "JSONL record at line {} missing type",
                        line_number + 1
                    ))
                })? {
                "session_meta" => {
                    version = required_u32(object, "version")?;
                    session_id = Some(required_string(object, "session_id")?);
                    created_at_ms = Some(required_u64(object, "created_at_ms")?);
                    updated_at_ms = Some(required_u64(object, "updated_at_ms")?);
                    // transpose 用于把 Option<Result> -> Result<Option<>>，这样可以更好的使用 ?
                    fork = object.get("fork").map(SessionFork::from_json).transpose()?;
                    workspace_root = object
                        .get("workspace_root")
                        .and_then(JsonValue::as_str)
                        .map(PathBuf::from);
                    model = object
                        .get("model")
                        .and_then(JsonValue::as_str)
                        .map(String::from);
                }
                "message" => {
                    let message_value = object.get("message").ok_or_else(|| {
                        SessionError::Format(format!(
                            "JSONL record at line {} missing message",
                            line_number + 1
                        ))
                    })?;
                    messages.push(ConversationMessage::from_json(message_value)?);
                }
                "compaction" => {
                    compaction = Some(SessionCompaction::from_json(&JsonValue::Object(
                        object.clone(),
                    ))?)
                }
                "prompt_history" => {
                    if let Some(entry) =
                        SessionPromptEntry::from_json_opt(&JsonValue::Object(object.clone()))
                    {
                        prompt_history.push(entry);
                    }
                }
                other => {
                    return Err(SessionError::Format(format!(
                        "unsupported JSONL record type at line {}: {other}",
                        line_number + 1
                    )))
                }
            }
        }

        let now = current_time_millis();
        Ok(Self {
            version,
            session_id: session_id.unwrap_or_else(generate_session_id),
            created_at_ms: created_at_ms.unwrap_or(now),
            updated_at_ms: updated_at_ms.unwrap_or(created_at_ms.unwrap_or(now)),
            messages,
            compaction,
            fork,
            workspace_root,
            prompt_history,
            last_health_check_ms: None,
            model,
            persistence: None,
        })
    }
}

// 清理过多的历史轮转日志文件，保留修改时间最新的几条
fn cleanup_rotated_logs(path: &Path) -> Result<(), SessionError> {
    // 如果父目录都没有，直接返回
    let Some(parent) = path.parent() else {
        return Ok(());
    };

    // 取去除掉后缀的文件名
    let stem = path
        .file_stem()
        .and_then(|value| value.to_str())
        .unwrap_or("session");

    // 对父目录下的所有文件遍历，找出符合格式的轮转文件
    let prefix = format!("{stem}.rot-");
    let mut rotated_paths = fs::read_dir(parent)?
        .filter_map(Result::ok)
        .map(|entry| entry.path())
        .filter(|entry_path| {
            entry_path
                .file_name()
                .and_then(|value| value.to_str())
                .is_some_and(|name| {
                    name.starts_with(&prefix)
                        && Path::new(name)
                            .extension()
                            .is_some_and(|ext| ext.eq_ignore_ascii_case("jsonl"))
                })
        })
        .collect::<Vec<_>>();

    // 按照修改时间排序，越旧的越排在前面
    rotated_paths.sort_by_key(|entry_path| {
        fs::metadata(entry_path)
            .and_then(|metadata| metadata.modified())
            .unwrap_or(UNIX_EPOCH)
    });

    // 计算应该删除的文件数量
    let remove_count = rotated_paths.len().saturating_sub(MAX_ROTATED_FILES);
    // 从旧到新开始删除
    for stale_path in rotated_paths.into_iter().take(remove_count) {
        fs::remove_file(stale_path)?;
    }
    Ok(())
}

// 写入临时文件后改名，避免影响到正在读取的用户，让用户看到中间态
fn write_atomic(path: &Path, contents: &str) -> Result<(), SessionError> {
    // 创建父目录
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }
    // 拼接得到一个临时文件路径
    let temp_path = temporary_path_for(path);
    // 把内容写到临时文件内
    fs::write(&temp_path, contents)?;
    // 把临时文件改名为 path
    fs::rename(temp_path, path)?;
    Ok(())
}

fn temporary_path_for(path: &Path) -> PathBuf {
    let file_name = path
        .file_name()
        .and_then(|value| value.to_str())
        .unwrap_or("session");
    // 在路径下拼接一个临时文件
    // path.with_file_name(...) 保留原来的目录部分，只把路径里的“文件名”替换成你传进去的新名字
    path.with_file_name(format!(
        "{file_name}.tmp-{}-{}",
        current_time_millis(),
        // 对一个原子计数器做 +1，返回“加之前”的旧值，避免同一毫秒内写两次
        // 保证这次原子读/写/加法本身不会被多个线程打坏
        // 但不保证它和其他内存读写之间的先后可见关系
        SESSION_ID_COUNTER.fetch_add(1, Ordering::Relaxed)
    ))
}

fn rotate_session_file_if_needed(path: &Path) -> Result<(), SessionError> {
    let Ok(metadata) = fs::metadata(path) else {
        return Ok(());
    };
    // 从元信息中获取到文件大小，判断是否到达归档尺寸
    if metadata.len() < ROTATE_AFTER_BYTES {
        return Ok(());
    }
    // 原文件改名归档
    let rotate_path = rotate_log_path(path);
    // 把原来的文件“改名/移动”到轮转后的新路径
    fs::rename(path, rotate_path)?;
    Ok(())
}

// 对 path 指向的文件归档
fn rotate_log_path(path: &Path) -> PathBuf {
    let stem = path
        // 取原文件名的 stem ，也就是不带扩展名的主文件名
        .file_stem()
        .and_then(|value| value.to_str())
        .unwrap_or("session");
    // 返回归档后的文件名
    path.with_file_name(format!("{stem}.rot-{}.jsonl", current_time_millis()))
}

// 把一条 ConversationMessage 包装成一条可持久化的 JSON 记录
fn message_record(message: &ConversationMessage) -> JsonValue {
    let mut object = BTreeMap::new();
    object.insert("type".to_string(), JsonValue::String("message".to_string()));
    object.insert("message".to_string(), message.to_json());
    JsonValue::Object(object)
}

impl SessionCompaction {
    fn from_json(value: &JsonValue) -> Result<Self, SessionError> {
        let object = value
            .as_object()
            .ok_or_else(|| SessionError::Format("compaction must be an object".to_string()))?;
        Ok(Self {
            count: required_u32(object, "count")?,
            removed_message_count: required_usize(object, "removed_message_count")?,
            summary: required_string(object, "summary")?,
        })
    }

    // 把 compaction 信息转换为 JsonValue
    pub fn to_jsonl_record(&self) -> Result<JsonValue, SessionError> {
        let mut object = BTreeMap::new();
        object.insert(
            "type".to_string(),
            JsonValue::String("compaction".to_string()),
        );
        object.insert(
            "count".to_string(),
            JsonValue::Number(i64::from(self.count)),
        );
        object.insert(
            "removed_message_count".to_string(),
            JsonValue::Number(i64_from_usize(
                self.removed_message_count,
                "removed_message_count",
            )?),
        );
        object.insert(
            "summary".to_string(),
            JsonValue::String(self.summary.clone()),
        );
        Ok(JsonValue::Object(object))
    }
}

impl SessionPromptEntry {
    fn from_json_opt(value: &JsonValue) -> Option<Self> {
        let object = value.as_object()?;
        let timestamp_ms = object
            .get("timestamp_ms")
            .and_then(JsonValue::as_i64)
            .and_then(|value| u64::try_from(value).ok)?;
        let text = object.get("text").and_then(JsonValue::as_str)?.to_string();
        Some(Self { timestamp_ms, text })
    }

    pub fn to_jsonl_record(&self) -> JsonValue {
        let mut object = BTreeMap::new();
        // 类型标记
        object.insert(
            "type".to_string(),
            JsonValue::String("prompt_history".to_string()),
        );
        object.insert(
            "timestamp_ms".to_string(),
            JsonValue::Number(i64::try_from(self.timestamp_ms).unwrap_or(i64::MAX)),
        );
        object.insert("text".to_string(), JsonValue::String(self.text.clone()));
        JsonValue::Object(object)
    }
}

impl ConversationMessage {
    // 从 json 中取出字段转换成 ConversationMessage
    fn from_json(value: &JsonValue) -> Result<Self, SessionError> {
        let object = value
            .as_object()
            .ok_or_else(|| SessionError::Format("message must be an object".to_string()))?;
        let role = match object
            .get("role")
            .and_then(JsonValue::as_str)
            // 如果是 Ok(value) ，继续往下执行，取出 value 进行枚举处理
            .ok_or_else(|| SessionError::Format("missing role".to_string()))?
        {
            "system" => MessageRole::System,
            "user" => MessageRole::User,
            "assistant" => MessageRole::Assistant,
            "tool" => MessageRole::Tool,
            other => {
                return Err(SessionError::Format(format!(
                    "unsupported message role: {other}"
                )))
            }
        };
        let blocks = object
            .get("blocks")
            .and_then(JsonValue::as_array)
            .ok_or_else(|| SessionError::Format("missing blocks".to_string()))?
            // 要先把数组引用变成迭代器，才能继续 .map(...)
            .iter()
            .map(ContentBlock::from_json)
            .collect::<Result<Vec<_>, _>>()?;
        // 把 Option<Result<T, E>> 转成 Result<Option<T>, E>
        let usage = object.get("usage").map(usage_from_json).transpose()?;
        Ok(Self {
            role,
            blocks,
            usage,
        })
    }
}

impl SessionFork {
    // 从 JsonValue 中逐个字段拼凑出 Session
    fn from_json(value: &JsonValue) -> Result<Self, SessionError> {
        let object = value
            .as_object()
            .ok_or_else(|| SessionError::Format("fork metadata must be an object".to_string()))?;
        Ok(Self {
            parent_session_id: required_string(object, "parent_session_id")?,
            branch_name: object
                .get("branch_name")
                .and_then(JsonValue::as_str)
                .map(ToOwned::to_owned),
        })
    }
}

// 使用当前时间和 counter 计算出一个唯一 id
fn generate_session_id() -> String {
    let millis = current_time_millis();
    // 计数器 + 1，它的返回值是“加之前”的旧值
    let counter = SESSION_ID_COUNTER.fetch_add(1, Ordering::Relaxed);
    format!("session-{millis}-{counter}")
}

impl ContentBlock {
    fn from_json(value: &JsonValue) -> Result<Self, SessionError> {
        let object = value
            .as_object()
            .ok_or_else(|| SessionError::Format("block must be an object".to_string()))?;
        match object
            .get("type")
            .and_then(JsonValue::as_str)
            .ok_or_else(|| SessionError::Format("missing block type".to_string()))?
        {
            "text" => Ok(Self::Text {
                text: required_string(object, "text")?,
            }),
            "thinking" => Ok(Self::Thinking {
                thinking: required_string(object, "thinking")?,
                signature: object
                    .get("signature")
                    .and_then(JsonValue::as_str)
                    .map(String::from),
            }),
            "tool_use" => Ok(Self::ToolUse {
                id: required_string(object, "id")?,
                name: required_string(object, "name")?,
                input: required_string(object, "input")?,
            }),
            "too_result" => Ok(Self::ToolResult {
                tool_use_id: required_string(object, "tool_use_id")?,
                tool_name: required_string(object, "tool_name")?,
                output: required_string(object, "output")?,
                is_error: object
                    .get("is_error")
                    .and_then(JsonValue::as_bool)
                    .ok_or_else(|| SessionError::Format("missing is_error".to_string()))?,
            }),
            other => Err(SessionError::Format(format!(
                "unsupported block type: {other}"
            ))),
        }
    }
}

fn usage_from_json(value: &JsonValue) -> Result<TokenUsage, SessionError> {
    let object = value
        .as_object()
        .ok_or_else(|| SessionError::Format("usage must be an object".to_string()))?;
    Ok(TokenUsage {
        input_tokens: required_u32(object, "input_tokens")?,
        output_tokens: required_u32(object, "output_tokens")?,
        cache_creation_input_tokens: required_u32(object, "cache_creation_input_tokens")?,
        cache_read_input_tokens: required_u32(object, "cache_read_input_tokens")?,
    })
}

fn required_string(
    object: &BTreeMap<String, JsonValue>,
    key: &str,
) -> Result<String, SessionError> {
    object
        .get(key)
        .and_then(JsonValue::as_str)
        // Option<&str> 转成 Option<String>
        .map(ToOwned::to_owned)
        .ok_or_else(|| SessionError::Format(format!("missing {key}")))
}

fn required_usize(object: &BTreeMap<String, JsonValue>, key: &str) -> Result<usize, SessionError> {
    let value = object
        .get(key)
        .and_then(JsonValue::as_i64)
        .ok_or_else(|| SessionError::Format(format!("missing {key}")))?;
    usize::try_from(value).map_err(|_| SessionError::Format(format!("{key} out of range")))
}

// 从 object 中获取 key，将其转换为 u32 类型
fn required_u32(object: &BTreeMap<String, JsonValue>, key: &str) -> Result<u32, SessionError> {
    let value = object
        .get(key)
        .and_then(JsonValue::as_i64)
        .ok_or_else(|| SessionError::Format(format!("missing {key}")))?;
    u32::try_from(value).map_err(|_| SessionError::Format(format!("{key} out of range")))
}

// 不是单纯地调用系统时间，而是保证返回值在整个进程里“严格递增”，即后一次调用一定大于前一次
fn current_time_millis() -> u64 {
    // 计算时间戳
    let wall_clock = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| u64::try_from(duration.as_millis()).unwrap_or(u64::MAX))
        .unwrap_or_default();
    let mut candidate = wall_clock;

    // 和 LAST_TIMESTAMP_MS 比较，保证最终的返回值一定是递增的
    // 如果 wall_clock > previous ，说明时间正常往前走，就用它
    // 如果 wall_clock <= previous ，说明出现了下面任一种情况：
    // - 同一毫秒内被多次调用
    // - 系统时钟精度不够
    // - 系统时间回拨了
    // 这时它会把返回值改成 previous + 1，保证递增
    loop {
        // Relaxed 保证读取到的值不会是正在修改的值，不会出现读裂的情况，保证值合法
        let previous = LAST_TIMESTAMP_MS.load(Ordering::Relaxed);
        if candidate <= previous {
            candidate = previous.saturating_add(1);
        }

        // 可能在读出来这段时间，值已经被修改过了，所以要做 CAS 检验
        match LAST_TIMESTAMP_MS.compare_exchange(
            previous,
            candidate,
            Ordering::SeqCst,
            Ordering::SeqCst,
        ) {
            Ok(_) => return candidate,
            // 如果失败，获得实际值
            Err(actual) => candidate = actual.saturating_add(1),
        }
    }
}

fn required_u64(object: &BTreeMap<String, JsonValue>, key: &str) -> Result<u64, SessionError> {
    let value = object
        .get(key)
        .ok_or_else(|| SessionError::Format(format!("missing {key}")))?;
    required_u64_from_value(value, key)
}

fn required_u64_from_value(value: &JsonValue, key: &str) -> Result<u64, SessionError> {
    let value = value
        .as_i64()
        .ok_or_else(|| SessionError::Format(format!("missing {key}")))?;
    u64::try_from(value).map_err(|_| SessionError::Format(format!("{key} out of range")))
}

fn i64_from_u64(value: u64, key: &str) -> Result<i64, SessionError> {
    i64::try_from(value)
        .map_err(|_| SessionError::Format(format!("{key} out of range for JSON number")))
}

fn i64_from_usize(value: usize, key: &str) -> Result<i64, SessionError> {
    i64::try_from(value)
        .map_err(|_| SessionError::Format(format!("{key} out of range for JSON number")))
}

// 把 path 从 &str 转换为 string
fn workspace_root_to_string(path: &Path) -> Result<String, SessionError> {
    path.to_str().map(ToOwned::to_owned).ok_or_else(|| {
        SessionError::Format(format!(
            "workspace_root is not valid UTF-8: {}",
            path.display()
        ))
    })
}
