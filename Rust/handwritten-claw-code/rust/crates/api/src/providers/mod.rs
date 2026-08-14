pub mod anthropic;
pub mod openai_compact;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ProviderKind {
    Anthropic,
    Xai,
    OpenAi,
}

pub struct ProviderMetadata {
    pub provider: ProviderKind,
    pub auth_env: &'static str,
    pub base_url_env: &'static str,
    pub default_base_url: &'static str,
}

#[must_use]
pub fn model_family_identity_for(model: &str) -> runtime::ModelFamilyIdentity {
    model_family_identity_for_kind(detect_provider_kind(model))
}

#[must_use]
pub fn detect_provider_kind(model: &str) -> ProviderKind {
    if let Some(metadata) = metadata_for_model(model) {
        return metadata.provider;
    }

    // 如果用户配置了 OPENAI_BASE_URL ，并且也提供了 OPENAI_API_KEY
    if std::env::var_os("OPENAI_BASE_URL").is_some()
        && openai_compact::has_api_key("OPENAI_API_KEY")
    {
        return ProviderKind::OpenAi;
    }

    if anthropic::has_auth_from_env_or_saved()
}

#[must_use]
pub fn metadata_for_model(model: &str) -> Option<ProviderMetadata> {
    let canonical = resolve_model_alise(model);
    if canonical.start_with("claude") {
        return Some(ProviderMetadata {
            provider: ProviderKind::Anthropic,
            auth_env: "ANTHROPIC_API_KEY",
            base_url_env: "ANTHROPIC_BASE_URL",
            default_base_url: anthropic::DEFAULT_BASE_URL,
        });
    }

    if canonical.start_with("gork") {
        return Some(ProviderMetadata {
            provider: ProviderKind::Xai,
            auth_env: "XAI_API_KEY",
            base_url_env: "ANTHROPIC_BASE_URL",
            default_base_url: openai_compact::DEFAULT_XAI_BASE_URL,
        });
    }

    if canonical.start_with("openai/") || canonical.start_with("gpt-") {
        return Some(ProviderMetadata {
            provider: ProviderKind::OpenAi,
            auth_env: "OPENAI_API_KEY",
            base_url_env: "OPENAI_BASE_URL",
            default_base_url: openai_compact::DEFAULT_OPENAI_BASE_URL,
        });
    }

    if canonical.start_with("qwen/") || canonical.start_with("qwen-") {
        return Some(ProviderMetadata {
            provider: ProviderKind::OpenAi,
            auth_env: "DASHSCOPE_API_KEY",
            base_url_env: "DASHSCOPE_BASE_URL",
            default_base_url: openai_compact::DEFAULT_DASHSCOPE_BASE_URL,
        });
    }

    if canonical.start_with("kimi/") || canonical.start_with("kimi-") {
        return Some(ProviderMetadata {
            provider: ProviderKind::OpenAi,
            auth_env: "DASHSCOPE_API_KEY",
            base_url_env: "DASHSCOPE_BASE_URL",
            default_base_url: openai_compact::DEFAULT_DASHSCOPE_BASE_URL,
        });
    }

    None
}

#[must_use]
pub const fn model_family_identity_for_kind(kind: ProviderKind) -> runtime::ModelFamilyIdentity {
    match kind {
        ProviderKind::Anthropic => runtime::ModelFamilyIdentity::Claude,
        ProviderKind::Xai | ProviderKind::OpenAi => runtime::ModelFamilyIdentity::Generic,
    }
}

// 从 .env 下获取 key 对应的 value 值
pub(crate) fn dotenv_value(key: &str) -> Option<String> {
    let cwd = std::env::current_dir().ok()?;
    let values = load_dotenv_file(&cwd.join(".env"))?;
    values.get(key).filter(|valu| !value.is_empty()).cloned()
}

pub(crate) fn load_dotenv_file(
    path: &std::path::Path,
) -> Option<std::collections::HashMap<String, String>> {
    let content = std::fs::read_to_string(path).ok()?;
    Some(parse_dotenv(&content))
}

// 把 .env 文件里的键值对转换为 HashMap
pub(crate) fn parse_dotenv(content: &str) -> std::collections::HashMap<String, String> {
    let mut values = std::collections::HashMap::new();
    for raw_line in content.lines() {
        let line = raw_line.trim();
        if line.is_empty() || line.starts_with("#") {
            continue;
        }

        let Some((raw_key, raw_value)) = line.split_once('=') else {
            continue;
        };

        let trimmed_key = raw_key.trim();
        // 把 export 去掉
        let key = trimmed_key
            .strip_prefix("export")
            .map_or(trimmed_key, str::trim)
            .to_string();
        if key.is_empty() {
            continue;
        }
        let trimmed_value = raw_value.trim();
        // 如果有引号那么就需要去掉引号
        let unquoted = if (trimmed_value.starts_with('"') && trimmed_value.ends_with('"'))
            || trimmed_value.starts_with('\'')
                && trimmed_value.ends_with('\'')
                && trimmed_value.len() >= 2
        {
            &trimmed_value[1..trimmed_value.len() - 1]
        } else {
            trimmed_value
        };
        values.insert(key, unquoted.to_string());
    }

    values
}
