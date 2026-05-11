use std::path::PathBuf;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ResolvedPermissionMode {
    ReadOnly,
    WorkspaceWrite,
    DangerFullAccess,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ConfigLoader {
    cwd: PathBuf,
    config_home: PathBuf,
}

impl ConfigLoader {
    #[must_use]
    pub fn new(cwd: impl Into<PathBuf>, config_home: impl Into<PathBuf>) -> Self {
        Self {
            cwd: cwd.into(),
            config_home: config_home.into(),
        }
    }

    #[must_use]
    pub fn default_for(cwd: impl Into<PathBuf>) -> Self {
        let cwd = cwd.into();
        let config_home = default_config_home();
        Self { cwd, config_home }
    }
}

#[must_use]
pub fn default_config_home() -> PathBuf {
    std::env::var_os("CLAW_CONFIG_HOME")
        .map(PathBuf::from)
        .or_else(|| std::env::var_os("HOME").map(|home| PathBuf::from(home).join(".claw")))
        .unwrap_or_else(|| PathBuf::from(".claw"))
}
