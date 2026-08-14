use crate::{
    session::{ContentBlock, MessageRole},
    ConversationMessage, Session,
};

const COMPACT_CONTINUATION_PREAMBLE: &str =
"This session is being continued from a previous conversation that ran out of context. The summary below covers the earlier portion of the conversation.\n\n";
const COMPACT_RECENT_MESSAGE_NOTE: &str = "Recent messages are preserved verbatim"; // 最近的消息没有被摘要化，而是原样保留
const COMPACT_DIRECT_RESUME_INSTRUCTION: &str = "Continue the conversation from where it left off without asking the user any further questions. Resume directly - do not ackonwledge the summary, do not recap what was happening, and do not preface with continuation text.";

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct CompactionConfig {
    pub preserve_recent_messages: usize,
    pub max_estimated_tokens: usize,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CompactionResult {
    pub summary: String,
    pub formatted_summary: String,
    pub compacted_session: Session,
    pub removed_message_count: usize,
}

#[must_use]
// 把一个过长的会话 session “压缩”成“摘要 + 最近几条原始消息”，以减少上下文长度，同时尽量保留继续对话所需的信息
pub fn compact_session(session: &Session, config: CompactionConfig) -> CompactionResult {
    // 判断会话 tokens 数是否足够长，满足配置的压缩域值
    if !should_compact(session, config) {
        return CompactionResult {
            summary: String::new(),
            formatted_summary: String::new(),
            compacted_session: session.clone(),
            removed_message_count: 0,
        };
    }

    // 取第一条消息，看它是不是之前压缩后插入的系统摘要
    let existing_summary = session
        .messages
        .first()
        .and_then(extract_existing_compacted_summary);
    // 如果已有旧摘要， compacted_prefix_len = 1
    let compacted_prefix_len = usize::from(existing_summary.is_some());
    // 按“保留最近 N 条”得到的原始边界，假如 session.messages.len() = 20，
    // config.preserve_recent_messages = 4，那么 raw_keep_from = 20 - 4 = 16
    let raw_keep_from = session
        .messages
        .len()
        .saturating_sub(config.preserve_recent_messages);

    // 修正 raw_keep_from 这个“原始保留边界”，避免把一次工具调用的两半拆开
    // 更具体地说，它要避免出现这种情况： 
    //  - 被保留的第一条消息是 ToolResult
    //  - 但对应的前一条 assistant 里的 ToolUse 被压缩掉了（ToolUse 和 ToolResult 应该一起保留）
    let keep_from = {
        // 初始起点，k 表示“保留区从哪一条消息开始”
        let mut k = raw_keep_from;

        loop {
            // k ==0 表示已经退到消息列表最开头了
            // 因为摘要只有一条消息，所以 compacted_prefix_len 取值只有 0 和 1
            if k == 0 || k <= compacted_prefix_len {
                break;
            }
            // 取第一条保留消息
            let first_preserved = &session.messages[k];
            // 判断它是不是 ToolResult
            let start_with_tool_result = first_preserved
                .blocks
                .first()
                .is_some_and(|b| matches!(b, ContentBlock::ToolResult { .. }));
            // 如果不是，就说明边界没切到“工具结果”上
            // 那就安全，停止调整
            if !start_with_tool_result {
                break;
            }
            // 接着看边界前一条消息
            let preceding = &session.messages[k - 1];
            // 检查它里面有没有 ToolUse
            let preceding_has_tool_use = preceding
                .blocks
                .iter()
                .any(|b| matches!(b, ContentBlock::ToolUse { .. }));
            // 如果有，那么说明说明边界刚好切在：
            // 前一条： ToolUse，当前条： ToolResult
            if preceding_has_tool_use {
                // 为了避免割裂，把边界往前退一格，把前面的 ToolUse 一起保留
                k = k.saturating_sub(1);
                break;
            }
            // 当前保留区第一条是 ToolResult
            // 但它前面那条也不是 ToolUse
            // 说明现在已经是“孤儿 ToolResult”了，边界前面可能还要继续找
            // 所以先把 k 再往前退一格
            // 下一轮循环继续检查新的 session.messages[k]
            k = k.saturating_sub(1);
        }
        k
    };
    // 待被压缩的消息，compacted_prefix_len 用来跳过“之前已经存在的摘要前缀”，避免重复压缩老摘要。
    let removed = &session.messages[compacted_prefix_len..keep_from];
    // 应该被保留的消息
    let preserved = session.messages[keep_from..].to_vec();
    // 合并新旧摘要
    let summary =
        merge_compact_summaries(existing_summary.as_deref(), &summarize_messages(removed));
    // 格式化一下摘要
    let formatted_summary = format_compact_summary(&summary);
    // 添加总结性说明性语句，告知摘要的存在
    let continuation = get_compact_continuation_message(&summary, true, !preserved.is_empty());

    // 构造一条新的会话，用于承载上面的摘要
    let mut compacted_message = vec![ConversationMessage {
        role: MessageRole::System,
        // 记录上面生成的消息
        blocks: vec![ContentBlock::Text { text: continuation }],
        usage: None,
    }];
    // 重新在后面加上被保留的消息
    compacted_message.extend(preserved);

    // 因为 session 是借用传入的，所以想要修改得自己克隆一个
    let mut compacted_session = session.clone();
    // 把压缩后的信息放入 session 中
    compacted_session.messages = compacted_message;
    // 记录压缩的原信息到 session 中
    compacted_session.record_compaction(summary.clone(), removed.len());

    CompactionResult {
        // 合并后的摘要文本
        summary,
        // 格式化后的摘要文本
        formatted_summary,
        // 完整的压缩 session
        compacted_session,
        // 被移除的 message 数量
        removed_message_count: removed.len(),
    }
}

// 在会话被 compact 之后，生成一条“继续对话用的系统消息”
// 这条消息会告诉模型：前面的长对话已经被压缩成摘要了，下面这段摘要就是之前上下文的替代品
// 同时它还会根据参数，追加一些控制性说明，比如“最近几条消息是否原样保留”和“不要再追问、直接续写”
pub fn get_compact_continuation_message(
    summary: &str,
    suppress_follow_up_question: bool,
    recent_message_preserved: bool,
) -> String {
    // 告诉模型接触的是摘要
    let mut base = format!(
        "{COMPACT_CONTINUATION_PREAMBLE}{}",
        format_compact_summary(summary)
    );

    // 最近的消息是否被保留
    if recent_message_preserved {
        base.push_str("\n\n");
        base.push_str(COMPACT_RECENT_MESSAGE_NOTE);
    }

    // 禁止模型向用户追问问题
    if suppress_follow_up_question {
        base.push('\n');
        base.push_str(COMPACT_DIRECT_RESUME_INSTRUCTION);
    }

    base
}

// 把内部生成的 compact summary，整理成更适合展示或继续注入上下文的文本格式
pub fn format_compact_summary(summary: &str) -> String {
    // 移除掉 analysis 标签
    let without_analysis = strip_tag_block(summary, "analysis");
    // 把 <summary> ... </summary> 替换成 Summary:\n...
    let formatted = if let Some(content) = extract_tag_block(&without_analysis, "summary") {
        without_analysis.replace(
            &format!("<summary>{content}</summary>"),
            &format!("Summary:\n{}", content.trim()),
        )
    } else {
        without_analysis
    };

    collapse_blank_lines(&formatted).trim().to_string()
}

// 把一组历史对话消息压缩成一段结构化摘要文本
fn summarize_messages(messages: &[ConversationMessage]) -> String {
    // 统计不同角色发送的消息数量
    let user_messages = messages
        .iter()
        .filter(|message| message.role == MessageRole::User)
        .count();
    let assistant_messages = messages
        .iter()
        .filter(|message| message.role == MessageRole::Assistant)
        .count();
    let tool_messages = messages
        .iter()
        .filter(|message| message.role == MessageRole::Tool)
        .count();

    // 收集所有的工具名，排序后去重
    let mut tool_names = messages
        .iter()
        .flat_map(|message| message.blocks.iter())
        .filter_map(|block| match block {
            ContentBlock::ToolUse { name, .. } => Some(name.as_str()),
            ContentBlock::ToolResult { tool_name, .. } => Some(tool_name.as_str()),
            ContentBlock::Text { .. } => None,
            ContentBlock::Thinking { .. } => None,
        })
        .collect::<Vec<_>>();
    tool_names.sort_unstable();
    tool_names.dedup();

    // 初始化输出摘要的头部，填入刚刚计算的长度
    let mut lines = vec![
        "<summary>".to_string(),
        "Conversation summary:".to_string(),
        format!(
            "- Scope: {} earlier messages compacted (user={}, assistant={}, tool={}).",
            messages.len(),
            user_messages,
            assistant_messages,
            tool_messages,
        ),
    ];

    // 如果用到了工具，填入工具的摘要信息
    if !tool_names.is_empty() {
        lines.push(format!("- Tools mentioned: {}", tool_names.join(", ")));
    }

    // 收集 user 最近的 3 条消息
    let recent_user_requests = collect_recent_role_summaries(messages, MessageRole::User, 3);
    // 如果用户有消息，写入总结中
    if !recent_user_requests.is_empty() {
        lines.push("- Recent user request:".to_string());
        lines.extend(
            recent_user_requests
                .into_iter()
                .map(|request| format!(" - {request}")),
        );
    }

    // 加入待处理工作进入总结中
    let pending_work = infer_pending_work(messages);
    if !pending_work.is_empty() {
        lines.push("- Pending work:".to_string());
        lines.extend(pending_work.into_iter().map(|item| format!(" - {item}")));
    }

    // 加入关键文件到总结中
    let key_files = collect_key_files(messages);
    if !key_files.is_empty() {
        lines.push(format!("- Key files referenced: {}", key_files.join(", ")));
    }

    // 取最近一条非空文本作为“当前在做什么”
    if let Some(current_work) = infer_current_work(messages) {
        lines.push(format!("- Current work: {current_work}"));
    }

    // 加入关键时间线
    lines.push("- Key timeline:".to_string());
    // 遍历每条 message，总结每个角色的对话 block，每条消息形成一行 timeline
    for message in messages {
        let role = match message.role {
            MessageRole::System => "system",
            MessageRole::User => "user",
            MessageRole::Assistant => "assistant",
            MessageRole::Tool => "tool",
        };
        let content = message
            .blocks
            .iter()
            .map(summarize_block)
            .collect::<Vec<_>>()
            .join(" | ");
        lines.push(format!("  - {role}: {content}"));
    }

    // 加上结尾，摘要结束
    lines.push("</summary>".to_string());
    lines.join("\n")
}

// 把一个 ContentBlock 转成适合放进摘要里的短字符串
fn summarize_block(block: &ContentBlock) -> String {
    // 对于不同的 block ，用不同的方式生成不同语句
    let raw = match block {
        ContentBlock::Text { text } => text.clone(),
        ContentBlock::Thinking { thinking, .. } => {
            format!("thinking ({} chars)", thinking.chars().count())
        }
        ContentBlock::ToolUse { name, input, .. } => format!("tool_use {name} ({input})"),
        ContentBlock::ToolResult {
            tool_name,
            output,
            is_error,
            ..
        } => format!(
            "tool_result {tool_name}: {}{output}",
            if *is_error { "error" } else { "" }
        ),
    };
    // 把语句进行截取
    truncate_summary(&raw, 160)
}

// 返回最近一条非空的文本内容，作为当前工作摘要
fn infer_current_work(messages: &[ConversationMessage]) -> Option<String> {
    messages
        .iter()
        .rev()
        .filter_map(first_text_block)
        .find(|text| !text.trim().is_empty())
        .map(|text| truncate_summary(text, 200))
}

// 从一组对话消息 messages 里，收集被提到的“关键文件路径”
fn collect_key_files(messages: &[ConversationMessage]) -> Vec<String> {
    let mut files = messages
        // 遍历所有消息
        .iter()
        // 遍历所有 block
        .flat_map(|message| message.blocks.iter())
        // 根据不同的 block 类型获取不同所需的文本
        .map(|block| match block {
            ContentBlock::Text { text } => text.as_str(),
            ContentBlock::ToolUse { input, .. } => input.as_str(),
            ContentBlock::ToolResult { output, .. } => output.as_str(),
            ContentBlock::Thinking { thinking, .. } => thinking.as_str(),
        })
        // 提取可能的文件路径
        .flat_map(extract_file_candidates)
        .collect::<Vec<_>>();
    // 排序去重
    files.sort();
    files.dedup();
    // 最多保留前 8 个文件
    files.into_iter().take(8).collect()
}

// 从一段文本 content 里提取“看起来像文件路径”的字符串
fn extract_file_candidates(content: &str) -> Vec<String> {
    content
        .split_whitespace()
        .filter_map(|token| {
            // 去掉 token 两端常见的标点符号
            let candidate = token.trim_matches(|char: char| {
                matches!(char, ',' | '.' | ';' | ')' | '(' | '"' | '\'' | '`')
            });
            // 包含 / 同时有特定的后缀，那么就认为他们是需要的路径
            if candidate.contains('/') && has_interesting_extension(candidate) {
                Some(candidate.to_string())
            } else {
                None
            }
        })
        .collect()
}

// 判断文件名后缀是不是代码或者信息文档，json 等等
fn has_interesting_extension(candidate: &str) -> bool {
    // 把字符串当路径处理
    std::path::Path::new(candidate)
        .extension()
        .and_then(|extension| extension.to_str())
        // 判断是否满足后缀要求
        .is_some_and(|extension| {
            ["rs", "ts", "tsx", "js", "json", "md"]
                .iter()
                .any(|expected| extension.eq_ignore_ascii_case(expected))
        })
}

// 从一组对话消息里， 粗略推断“还有哪些待办/未完成工作”
// 没有依靠智能，而是靠最近几条文本消息里的关键词匹配
fn infer_pending_work(messages: &[ConversationMessage]) -> Vec<String> {
    messages
        .iter()
        .rev()
        // 获取第一个 text block
        .filter_map(first_text_block)
        // 匹配关键字
        .filter(|text| {
            let lowered = text.to_ascii_lowercase();
            lowered.contains("todo")
                || lowered.contains("next")
                || lowered.contains("pending")
                || lowered.contains("follow up")
                || lowered.contains("remaining")
        })
        .take(3)
        // 获取摘要内容
        .map(|text| truncate_summary(text, 160))
        .collect::<Vec<_>>()
        .into_iter()
        .rev()
        .collect()
}

// 从 messages 中取出特定 role 的消息
fn collect_recent_role_summaries(
    messages: &[ConversationMessage],
    role: MessageRole,
    limit: usize,
) -> Vec<String> {
    messages
        .iter()
        .filter(|message| message.role == role)
        // 从最新的消息开始看
        .rev()
        // 对每条消息提取第一个非空文本块
        .filter_map(|message| first_text_block(message))
        // 不是全取
        .take(limit)
        // 把每条文本裁成最多 160 个字符的摘要
        .map(|text| truncate_summary(text, 160))
        .collect::<Vec<_>>()
        .into_iter()
        // 重新翻转回来
        .rev()
        .collect()
}

// 如果超过上限，就截取前 max_chars 个字符，并在结尾补一个省略号 …
fn truncate_summary(content: &str, max_chars: usize) -> String {
    if content.chars().count() <= max_chars {
        return content.to_string();
    }
    let mut truncated = content.chars().take(max_chars).collect::<String>();
    truncated.push('…');
    truncated
}

// 把“旧的压缩摘要”和“这次新生成的摘要”合并成一个新的统一摘要字符串
fn merge_compact_summaries(existing_summary: Option<&str>, new_summary: &str) -> String {
    // 无需合并直接返回
    let Some(existing_summary) = existing_summary else {
        return new_summary.to_string();
    };

    // 拿去之前摘要的重点信息
    let previous_highlights = extract_summary_highlights(existing_summary);
    // 整理一下新摘要的格式
    let new_formatted_summary = format_compact_summary(new_summary);
    // 拿取新摘要的重点信息
    let new_highlights = extract_summary_highlights(&new_formatted_summary);
    // 拿取新摘要的 timeline 信息
    let new_timeline = extract_summary_timeline(&new_formatted_summary);

    // 合并摘要的开头
    let mut lines = vec!["<summary>".to_string(), "Conversation summary".to_string()];

    // 旧摘要的重点内容，就放到 - Previously compacted context: 下面
    if !previous_highlights.is_empty() {
        lines.push("- Previously compacted context:".to_string());
        lines.extend(
            previous_highlights
                .into_iter()
                .map(|line| format!("  {line}")),
        );
    }

    // 新摘要的普通重点内容，就放到 - Newly compacted context: 下面
    if !new_highlights.is_empty() {
        lines.push("- Newly compacted context:".to_string());
        lines.extend(new_timeline.into_iter().map(|line| format!("  {line}")));
    }

    // 新摘要里有时间线内容，就放到 - Key timeline: 下面
    if !new_timeline.is_empty() {
        lines.push("- Key timeline:".to_string());
        lines.extend(new_timeline.into_iter().map(|line| format!("  {line}")));
    }

    lines.push("</summary>".to_string());
    lines.join("\n")
}

// 从一段压缩摘要里提取“高层重点信息”，不拿 timeline 信息
fn extract_summary_highlights(summary: &str) -> Vec<String> {
    let mut lines = Vec::new();
    let mut in_timeline = false;

    for line in format_compact_summary(summary).lines() {
        let trimmed = line.trim_end();
        // 跳过空行和标题行
        if trimmed.is_empty() || trimmed == "Summary" || trimmed == "Conversation summary:" {
            continue;
        }
        // 遇到- Key timeline: ，就把 in_timeline 设为 true
        if trimmed == "-Key timeline:" {
            in_timeline = true;
            continue;
        }
        // 进入 timeline 区域后，后面的行全部忽略
        if in_timeline {
            continue;
        }
        // 在 timeline 之前的普通行，收集进 lines
        lines.push(trimmed.to_string());
    }
    lines
}

pub fn format_compact_summary(summary: &str) -> String {
    let without_analysis = strip_tag_block(summary, "analysis");
    let formatted = if let Some(content) = extract_tag_block(&without_analysis, "summary") {
        without_analysis.replace(
            &format!("<summary>{content}</summary>"),
            &format!("Summary:\n{}", content.trim()),
        )
    } else {
        without_analysis
    };

    collapse_blank_lines(&formatted).trim().to_string()
}

// 移除掉特定的 tag
fn strip_tag_block(content: &str, tag: &str) -> String {
    let start = format!("<{tag}>");
    let end = format!("</{tag}>");
    if let (Some(start_index), Some(end_index_rel)) = (content.find(&start), content.find(&end)) {
        let end_index = end_index_rel + end.len();
        let mut stripped = String::new();
        stripped.push_str(&content[..start_index]);
        stripped.push_str((&content[end_index..]));
        stripped
    } else {
        content.to_string()
    }
}

fn collapse_blank_lines(content: &str) -> String {
    let mut result = String::new();
    let mut last_blank = false;
    for line in content.lines() {
        let is_blank = line.trim().is_empty();
        if is_blank && last_blank {
            continue;
        }
        result.push_str(line);
        result.push('\n');
        last_blank = is_blank;
    }
    result
}

// 提取被 tag 包裹的内容
fn extract_tag_block(content: &str, tag: &str) -> Option<String> {
    let start = format!("<{tag}>");
    let end = format!("</{tag}>");
    let start_index = content.find(&start) + start.len();
    let end_index = content[start_index..].find(&end)? + start_index;
    Some(content[start_index..end_index].to_string())
}

// 提取摘要中的时间线信息
fn extract_summary_timeline(summary: &str) -> Vec<String> {
    let mut lines = Vec::new();
    let mut in_timeline = false;

    for line in format_compact_summary(summary).lines() {
        let trimmed = line.trim_end();
        // 遇到 - Key timeline:，那么就开始收集
        if trimmed == "- Key timeline:" {
            in_timeline = true;
            continue;
        }
        if !in_timeline {
            continue;
        }
        if trimmed.is_empty() {
            break;
        }
        lines.push(trimmed.to_string());
    }
    lines
}

// 判断当前会话 session 是否已经“长到需要做 compact”
pub fn should_compact(session: &Session, config: CompactionConfig) -> bool {
    let start = compacted_summary_prefix_len(session);
    // 跳过压缩消息
    let compactable = &session.messages[start..];
    // 判断消息总数是否超过 "最近保留的尾部消息数"
    // 总 token 数量是否超过配置的数量
    compactable.len() > config.preserve_recent_messages
        && compactable
            .iter()
            .map(estimate_message_tokens)
            .sum::<usize>()
            >= config.max_estimated_tokens
}

// 粗略估算一条 ConversationMessage 大概会占多少 token，用一个很便宜的启发式规则做近似统计
// 考虑 4 个字符约等于一个 token
fn estimate_message_tokens(message: &ConversationMessage) -> usize {
    message
        .blocks
        .iter()
        .map(|block| match block {
            ContentBlock::Text { text } => text.len() / 4 + 1,
            ContentBlock::ToolUse { name, input, .. } => (name.len() + input.len()) / 4 + 1,
            ContentBlock::ToolResult {
                tool_name, output, ..
            } => (tool_name.len() + output.len()) / 4 + 1,
            ContentBlock::Thinking {
                thinking,
                signature,
            } => thinking.len() / 4 + signature.as_ref().map_or(0, |value| value.len() / 4 + 1),
        })
        .sum()
}

// 判断当前 session 的第一条消息是不是“已经压缩好的摘要消息”
fn compacted_summary_prefix_len(session: &Session) -> usize {
    usize::from(
        session
            .messages
            .first()
            .and_then(extract_existing_compacted_summary)
            .is_some(),
    )
}

// 从一条 ConversationMessage 里，尝试提取“已经压缩过的历史摘要”
fn extract_existing_compacted_summary(message: &ConversationMessage) -> Option<String> {
    // 如果检查消息的角色不是 System，那么直接报错
    if message.role != MessageRole::System {
        return None;
    }

    // 取出消息里的第一块文本块
    let text = first_text_block(message)?;
    // 去掉前导说明，把“这是一段上下文续接摘要”的提示头去掉
    let summary = text.strip_prefix(COMPACT_CONTINUATION_PREAMBLE)?;
    // 把“最近消息按原文保留”这一段说明从摘要文本里裁掉
    let summary = summary
        .split_once(&format!("\n\n{COMPACT_RECENT_MESSAGE_NOTE}"))
        .map_or(summary, |(value, _)| value);
    // 把“直接续接，不要加过渡话术”这类指令也剥离掉
    let summary = summary
        .split_once(&format!("\n{COMPACT_DIRECT_RESUME_INSTRUCTION}"))
        .map_or(summary, |(value, _)| value);
    // 返回真正，不加指令的摘要内容
    Some(summary.trim().to_string())
}

// 遍历 message 的 block，返回第一个 text block
fn first_text_block(message: &ConversationMessage) -> Option<&str> {
    message.blocks.iter().find_map(|block| match block {
        ContentBlock::Text { text } if !text.trim().is_empty() => Some(text.as_str()),
        // .. ，表示“其余字段我不关心，全部忽略”
        ContentBlock::ToolUse { .. }
        | ContentBlock::ToolResult { .. }
        | ContentBlock::Thinking { .. }
        | ContentBlock::Text { .. } => None,
    })
}
