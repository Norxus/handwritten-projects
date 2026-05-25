use std::{collections::BTreeMap, path::Path};

use crate::{json::JsonValue, ConfigError};

const TOP_LEVEL_FIELDS: &[FieldSpec] = &[
    FieldSpec {
        name: "$schema",
        expected: FieldType::String,
    },
    FieldSpec {
        name: "model",
        expected: FieldType::String,
    },
    FieldSpec {
        name: "hooks",
        expected: FieldType::Object,
    },
    FieldSpec {
        name: "permissions",
        expected: FieldType::Object,
    },
    FieldSpec {
        name: "permissionMode",
        expected: FieldType::String,
    },
    FieldSpec {
        name: "mcpServers",
        expected: FieldType::Object,
    },
    FieldSpec {
        name: "oauth",
        expected: FieldType::Object,
    },
    FieldSpec {
        name: "enabledPlugins",
        expected: FieldType::Object,
    },
    FieldSpec {
        name: "plugins",
        expected: FieldType::Object,
    },
    FieldSpec {
        name: "sandbox",
        expected: FieldType::Object,
    },
    FieldSpec {
        name: "env",
        expected: FieldType::Object,
    },
    FieldSpec {
        name: "aliases",
        expected: FieldType::Object,
    },
    FieldSpec {
        name: "providerFallbacks",
        expected: FieldType::Object,
    },
    FieldSpec {
        name: "trustedRoots",
        expected: FieldType::StringArray,
    },
];

struct DeprecatedField {
    name: &'static str,
    replacement: &'static str,
}

const DEPRECATED_FIELDS: &[DeprecatedField] = &[
    DeprecatedField {
        name: "permissionMode",
        replacement: "permissions.defaultMode",
    },
    DeprecatedField {
        name: "enabledPlugins",
        replacement: "plugins.enabled",
    },
];

const HOOKS_FIELDS: &[FieldSpec] = &[
    FieldSpec {
        name: "PreToolUse",
        expected: FieldType::StringArray,
    },
    FieldSpec {
        name: "PostToolUse",
        expected: FieldType::StringArray,
    },
    FieldSpec {
        name: "PostToolUseFailure",
        expected: FieldType::StringArray,
    },
];

const PERMISSIONS_FIELDS: &[FieldSpec] = &[
    FieldSpec {
        name: "defaultMode",
        expected: FieldType::String,
    },
    FieldSpec {
        name: "allow",
        expected: FieldType::StringArray,
    },
    FieldSpec {
        name: "deny",
        expected: FieldType::StringArray,
    },
    FieldSpec {
        name: "ask",
        expected: FieldType::StringArray,
    },
];

const PLUGINS_FIELDS: &[FieldSpec] = &[
    FieldSpec {
        name: "enabled",
        expected: FieldType::Object,
    },
    FieldSpec {
        name: "externalDirectories",
        expected: FieldType::StringArray,
    },
    FieldSpec {
        name: "installRoot",
        expected: FieldType::String,
    },
    FieldSpec {
        name: "registryPath",
        expected: FieldType::String,
    },
    FieldSpec {
        name: "bundledRoot",
        expected: FieldType::String,
    },
    FieldSpec {
        name: "maxOutputTokens",
        expected: FieldType::Number,
    },
];

const SANDBOX_FIELDS: &[FieldSpec] = &[
    FieldSpec {
        name: "enabled",
        expected: FieldType::Bool,
    },
    FieldSpec {
        name: "namespaceRestrictions",
        expected: FieldType::Bool,
    },
    FieldSpec {
        name: "networkIsolation",
        expected: FieldType::Bool,
    },
    FieldSpec {
        name: "filesystemMode",
        expected: FieldType::String,
    },
    FieldSpec {
        name: "allowedMounts",
        expected: FieldType::StringArray,
    },
];

const OAUTH_FIELDS: &[FieldSpec] = &[
    FieldSpec {
        name: "clientId",
        expected: FieldType::String,
    },
    FieldSpec {
        name: "authorizeUrl",
        expected: FieldType::String,
    },
    FieldSpec {
        name: "tokenUrl",
        expected: FieldType::String,
    },
    FieldSpec {
        name: "callbackPort",
        expected: FieldType::Number,
    },
    FieldSpec {
        name: "manualRedirectUrl",
        expected: FieldType::String,
    },
    FieldSpec {
        name: "scopes",
        expected: FieldType::StringArray,
    },
];

// 拒绝不支持的格式文件， eg.toml
pub fn check_unsupported_format(file_path: &Path) -> Result<(), ConfigError> {
    if let Some(ext) = file_path.extension().and_then(|e| e.to_str()) {
        if ext.eq_ignore_ascii_case("toml") {
            return Err(ConfigError::Parse(format!(
                "{}: TOML config files are not supported. Use JSON (settings.json) instead",
                file_path.display()
            )));
        }
    }

    Ok(())
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ValidationResult {
    pub errors: Vec<ConfigDiagnostic>,
    pub warnings: Vec<ConfigDiagnostic>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ConfigDiagnostic {
    pub path: String,
    pub field: String,
    pub line: Option<usize>,
    pub kind: DiagnosticKind,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum DiagnosticKind {
    UnknownKey {
        suggestion: Option<String>,
    },
    WrongType {
        expected: &'static str,
        got: &'static str,
    },
    Deprecated {
        replacement: &'static str,
    },
}

struct FieldSpec {
    name: &'static str,
    expected: FieldType,
}

enum FieldType {
    String,
    Bool,
    Object,
    StringArray,
    Number,
}

impl FieldType {
    fn label(self) -> &'static str {
        match self {
            Self::String => "a string",
            Self::Bool => "a boolean",
            Self::Object => "an object",
            Self::StringArray => "an array of strings",
            Self::Number => "a number",
        }
    }

    fn matches(self, value: &JsonValue) -> bool {
        match self {
            // is_some() ：如果读取成功，返回 true ；失败返回 false
            Self::String => value.as_str().is_some(),
            Self::Bool => value.as_bool().is_some(),
            Self::Object => value.as_object().is_some(),
            // 如果是 Some(x) ，就用你传进去的条件函数检查 x，如果是 None ，直接返回 false
            Self::StringArray => value
                .as_array()
                .is_some_and(|arr| arr.iter().all(|v| v.as_str().is_some())),
            Self::Number => value.as_i64().is_some(),
        }
    }
}

pub fn validate_config_file(
    object: &BTreeMap<String, JsonValue>,
    source: &str,
    file_path: &Path,
) -> ValidationResult {
    let path_display = file_path.display().to_string();
    let mut result = validate_object_keys(object, TOP_LEVEL_FIELDS, "", source, &path_display);

    for deprecated in DEPRECATED_FIELDS {
        if object.contains_key(deprecated.name) {
            result.warnings.push(ConfigDiagnostic {
                path: path_display.clone(),
                field: deprecated.name.to_string(),
                line: find_key_line(source, deprecated.name),
                kind: DiagnosticKind::Deprecated {
                    // 替换的选项
                    replacement: deprecated.replacement,
                },
            });
        }
    }

    //
    if let Some(hooks) = object.get("hooks").and_then(JsonValue::as_object) {
        result.merge(validate_object_keys(
            hooks,
            HOOKS_FIELDS,
            "hooks",
            source,
            &path_display,
        ));
    }

    if let Some(permissions) = object.get("permissions").and_then(JsonValue::as_object) {
        result.merge(validate_object_keys(
            permissions,
            PERMISSIONS_FIELDS,
            "permissions",
            source,
            &path_display,
        ));
    }

    if let Some(plugins) = object.get("plugins").and_then(JsonValue::as_object) {
        result.merge(validate_object_keys(
            plugins,
            PLUGINS_FIELDS,
            "plugins",
            source,
            &path_display,
        ));
    }
    if let Some(sandbox) = object.get("sandbox").and_then(JsonValue::as_object) {
        result.merge(validate_object_keys(
            sandbox,
            SANDBOX_FIELDS,
            "sandbox",
            source,
            &path_display,
        ));
    }
    if let Some(oauth) = object.get("oauth").and_then(JsonValue::as_object) {
        result.merge(validate_object_keys(
            oauth,
            OAUTH_FIELDS,
            "oauth",
            source,
            &path_display,
        ));
    }

    result
}

// 按给定 schema 校验一个 JSON 对象里的所有 key 是否合法，并顺带检查已知字段的值类型是否正确
// 如果都正确，那么返回一个“空结果”
fn validate_object_keys(
    object: &BTreeMap<String, JsonValue>,
    known_fields: &[FieldSpec],
    prefix: &str,
    source: &str,
    path_display: &str,
) -> ValidationResult {
    let mut result = ValidationResult {
        errors: Vec::new(),
        warnings: Vec::new(),
    };

    let known_names: Vec<&str> = known_fields.iter().map(|f| f.name).collect();

    for (key, value) in object {
        // 方便进行排查，eg. hooks.pre_run
        let field_path = if prefix.is_empty() {
            key.clone()
        } else {
            format!("{prefix}.{key}")
        };

        // 如果是 know_fields 中的名字，那么检查是否符合类型约束
        if let Some(spec) = known_fields.iter().find(|f| f.name == key) {
            // 判断 value 是否是 spec 的类型
            if !spec.expected.matches(value) {
                result.errors.push(ConfigDiagnostic {
                    path: path_display.to_string(),
                    field: field_path,
                    line: find_key_line(source, key),
                    kind: DiagnosticKind::WrongType {
                        expected: spec.expected.label(),
                        got: json_type_label(value),
                    },
                });
            }
        // 如果是已废弃字段，跳过
        } else if DEPRECATED_FIELDS.iter().any(|d| d.name == key) {
        } else {
            // unknown key，那么就找一个编辑距离最小的可选项
            let suggestion = suggest_field(key, &known_names);
            result.errors.push(ConfigDiagnostic {
                path: path_display.to_string(),
                field: field_path,
                line: find_key_line(source, key),
                kind: DiagnosticKind::UnknownKey { suggestion },
            })
        }
    }

    result
}

fn suggest_field(input: &str, candidates: &[&str]) -> Option<String> {
    let input_lower = input.to_ascii_lowercase();
    candidates
        .iter()
        .filter_map(|candidate| {
            // 找编辑距离小于 3 的候选，包装成 (distance, *candidate)
            let distance = simple_edit_distance(&input_lower, &candidate.to_ascii_lowercase());
            (distance <= 3).then_some((distance, *candidate))
        })
        .min_by_key(|distance, _| *distance)
        .map(|(_, name)| name.to_string())
}

fn simple_edit_distance(left: &str, right: &str) -> usize {
    if left.is_empty() {
        return right.len();
    }

    if right.is_empty() {
        return left.len();
    }

    let right_chars: Vec<char> = right.chars().collect();
    // 上一行 DP 结果
    let mut previous: Vec<usize> = (0..=right_chars.len()).collect();
    // 当前行 DP 结果
    let mut current = vec![0; right_chars.len() + 1];

    for (left_index, left_char) in left.chars().enumerate() {
        // left 的前 left_index + 1 个字符，变成空串的代价
        // 之所以要加 1，是因为数组索引从 0 开始
        current[0] = left_index + 1;
        for (right_index, right_char) in right_chars.iter().enumerate() {
            // 如果字符不匹配，那 left 可以通过三种方式变化
            // - 删除：表示先把 left 前 i-1 个字符变成 right 前 j 个字符, 再把当前这个 left_char 删掉
            // - 插入：表示先把 left 前 i 个字符变成 right 前 j-1 个字符, 再插入当前这个 right_char
            // - 替换：表示先把 left 前 i-1 个字符变成 right 前 j-1 个字符，再看当前两个字符
            let cost = usize::from(left_char != *right_char);
            current[right_index + 1] = (previous[right_index + 1] + 1)
                .min(current[right_index] + 1)
                .min(previous[right_index] + cost);
        }
        // 当前行变成上一行
        previous.clone_from(&current);
    }

    previous[right_chars.len()]
}

// 从 source 里找出 key 第一次出现的行号
fn find_key_line(source: &str, key: &str) -> Option<usize> {
    let needle = format!("\"{key}\"");
    let mut search_start = 0;
    while let Some(offset) = source[search_start..].find(&needle) {
        // 绝对位置（相对于 0）
        let absolute = search_start + offset;
        let after = absolute + needle.len();

        // key 后面的字符需要是 : 才是需要的位置
        if source[after..].chars().find(|ch| !ch.is_ascii_whitespace()) == Some(':') {
            // 统计当前 key 前面有多少个 \n，就是多少行
            return Some(source[..absolute].chars().filter(|&ch| ch == '\n').count() + 1);
        }
        // 不然从新位置开始搜索，避免一直卡在这个位置
        search_start = after;
    }
    None
}

fn json_type_label(value: &JsonValue) -> &'static str {
    match value {
        JsonValue::Null => "null",
        JsonValue::Bool(_) => "a boolean",
        JsonValue::Number(_) => "a number",
        JsonValue::String(_) => "a string",
        JsonValue::Array(_) => "an array",
        JsonValue::Object(_) => "an object",
    }
}
