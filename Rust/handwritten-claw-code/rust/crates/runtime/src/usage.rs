use crate::{usage, Session, TokenUsage};

pub struct UsageTracker {
    lastest_run: TokenUsage,
    cumulative: TokenUsage,
    truns: u32,
}

impl UsageTracker {
    #[must_use]
    pub fn new() -> Self {
        Self::default()
    }

    #[must_use]
    pub fn from_session(session: &Session) -> Self {
        let mut tracker = Self::new();
        for message in &session.messages {
            if let Some(usage) = message.usage {
                tracker.record(usage);
            }
        }
        tracker
    }

    pub fn record(&mut self, usage: TokenUsage) {
        // 最近一轮 token 的使用量
        self.lastest_run = usage;
        // 整个 session 累计 token 使用量
        self.cumulative.input_tokens += usage.input_tokens;
        self.cumulative.output_tokens += usage.output_tokens;
        self.cumulative.cache_creation_input_tokens += usage.cache_creation_input_tokens;
        self.cumulative.cache_read_input_tokens  += usage.cache_read_input_tokens;
        // 记录统计了几次
        self.truns += 1;
    }

    #[must_use]
    pub fn cumulative_usage(&self) -> TokenUsage {
        self.cumulative
    }
}
