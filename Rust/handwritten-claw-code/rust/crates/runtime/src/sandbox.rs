use std::default;

use serde::{Deserialize, Serialize};

// 只要 每个字段类型本身都实现了 Default，- 编译器就会自动为整个结构体生成 Default
// 生成方式是：对每个字段分别调用它自己的 default()
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq, Default)]
pub struct SandboxConfig {
    pub enabled: Option<bool>,
    pub namespace_restrictions: Option<bool>,
    pub network_isolation: Option<bool>,
    pub filesystem_mode: Option<FilesystemIsolationMode>,
    pub allowed_mounts: Vec<String>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, Default)]
#[serde(rename_all = "kebab-case")]
pub enum FilesystemIsolationMode {
    Off,
    // 当代码调用 FilesystemIsolationMode::default() 时，返回 WorkspaceOnly
    #[default]
    WorkspaceOnly,
    AllowList,
}
