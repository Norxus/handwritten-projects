use std::collections::BTreeMap;
use std::{env, fs};
use std::path::{Path, PathBuf};
use serde_json::{json, Value};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CommandRegistry {
    entries: Vec<CommandManifestEntry>,
}

impl CommandRegistry {

    #[must_use]
    pub fn new(entries: Vec<CommandManifestEntry>) -> Self {
        Self { entries }
    }

    pub fn entries(&self) -> &[CommandManifestEntry] {
        &self.entries
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CommandManifestEntry {
    pub name: String,
    pub source: CommandSource,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum CommandSource {
    Builtin,
    InternalOnly,
    FeatureGated,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
enum DefinitionSource {
    ProjectClaw,
    ProjectCodex,
    ProjectClaude,
    UserClawConfigHome,
    UserCodexHome,
    UserClaw,
    UserCodex,
    UserClaude,
}

impl DefinitionSource {
    fn report_scope(self) -> DefinitionScope  {
        match self {
            Self::ProjectClaw | Self::ProjectCodex | Self::ProjectClaude => {
                DefinitionScope::Project
            }
            Self::UserClawConfigHome | Self::UserCodexHome => {
                DefinitionScope::UserConfigHome
            }
            Self::UserClaw | Self::UserCodex | Self::UserClaude => {
                DefinitionScope::UserHome
            }
        }
    }

    fn label(self) -> &'static str {
        self.report_scope().label()
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct AgentSummary {
    name: String,
    description: Option<String>,
    model: Option<String>,
    reasoning_effort: Option<String>,
    source: DefinitionSource,
    shadowed_by: Option<DefinitionSource>,
}

pub fn handle_agents_slash_command(args: Option<&str>, cwd: &Path) -> std::io::Result<String> {
    if let Some(args) = normalize_optional_args(args) {
        if let Some(help_path) = help_path_from_args(args) {
            return Ok(match help_path.as_slice() {
                [] => render_agents_usage(None),
                _ => render_agents_usage(Some(&help_path.join(" "))),
            })
        }
    }

    match normalize_optional_args(args) {
       None | Some("list") => {
           let roots = discover_definition_roots(cwd, "agents");
           let agents = load_agents_from_roots(&roots)?;
           Ok(render_agents_report(&agents))
       }
        Some(args) if is_help_arg(args) => Ok(render_agents_usage(None)),
        Some(args) => Ok(render_agents_usage(Some(args))),
    }
}

fn render_agents_report(agents: &[AgentSummary]) -> String {
    if agents.is_empty() {
       return "No agents found".to_string();
    }

    let total_active = agents
        .iter()
        .filter(|agent| agent.shadowed_by.is_none())
        .count();

    let mut lines  = vec![
        "Agents".to_string(),
        format!("   {} active agents", total_active),
        String::new(),
    ];

    for scope in [
        DefinitionScope::Project,
        DefinitionScope::UserConfigHome,
        DefinitionScope::UserHome,
    ] {
        let group = agents
            .iter()
            .filter(|agent|agent.source.report_scope() == scope)
            .collect::<Vec<_>>();

        if group.is_empty() {
            continue;
        }

        lines.push(format!("{}:", scope.label()));
        for agent in group {
            let detail = agent_detail(agent);
            match agent.shadowed_by {
                Some(winner) => lines.push(format!("  (shadowed by {}) {detail}", winner.label())),
                None => lines.push(format!("  {detail}")),
            }
        }
        lines.push(String::new());
    }
    lines.join("\n").trim_end().to_string()
}

fn agent_detail(agent: &AgentSummary) -> String {
    let mut parts = vec![agent.name.clone()];
    if let Some(description) = &agent.description {
        parts.push(description.clone());
    }
    if let Some(model) = &agent.model {
       parts.push(model.clone());
    }

    if let Some(ref reasoning) = agent.reasoning_effort {
        parts.push(reasoning.clone());
    }

    parts.join(" · ")
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
enum DefinitionScope {
    Project,
    UserConfigHome,
    UserHome,
}

impl DefinitionScope {
    fn label(self) -> &'static str {
        match self {
            Self::Project => "Project roots",
            Self::UserConfigHome => "User config roots",
            Self::UserHome => "User home roots",
        }
    }
}

fn render_agents_usage(unexpected: Option<&str>) -> String {
   let mut lines = vec![
       "Agents".to_string(),
       "  Usage            /agents [list|help]".to_string(),
       "  Direct CLI       claw agents".to_string(),
       "  Sources          .claw/agents, ~/.claw/agents, $CLAW_CONFIG_HOME/agents".to_string(),
   ];
    if let Some(args) = unexpected {
        lines.push(format!("Unexpected  {}", args));
    }
    lines.join("\n")
}

fn discover_definition_roots(cwd: &Path, leaf: &str) -> Vec<(DefinitionSource, PathBuf)> {
    let mut roots = Vec::new();

    for ancestor in cwd.ancestors() {
        push_uniq_root(
            &mut roots,
            DefinitionSource::ProjectClaw,
            ancestor.join(".claw").join(leaf)
        );
        push_uniq_root(
            &mut roots,
            DefinitionSource::ProjectCodex,
            ancestor.join(".codex").join(leaf)
        );
        push_uniq_root(
            &mut roots,
            DefinitionSource::ProjectClaude,
            ancestor.join(".claude").join(leaf)
        );
    }

    if let Ok(claw_config_home) = env::var("CLAW_CONFIG_HOME") {
        push_uniq_root(
            &mut roots,
            DefinitionSource::UserClawConfigHome,
            PathBuf::from(claw_config_home).join(leaf)
        );
    }

    if let Ok(codex_home) = env::var("CODEX_HOME") {
        push_uniq_root(
            &mut roots,
            DefinitionSource::UserCodexHome,
            PathBuf::from(codex_home).join(leaf)
        );
    }

    if let Ok(claude_config_dir) = env::var("CLAUDE_CONFIG_DIR") {
        push_uniq_root(
            &mut roots,
            DefinitionSource::UserClaude,
            PathBuf::from(claude_config_dir).join(leaf)
        );
    }

    if let Some(home) = env::var_os("HOME") {
        let home = PathBuf::from(home);
        push_uniq_root(
            &mut roots,
            DefinitionSource::UserClaw,
            home.join(".claw").join(leaf)
        );
        push_uniq_root(
            &mut roots,
            DefinitionSource::UserCodex,
            home.join(".codex").join(leaf)
        );
        push_uniq_root(
            &mut roots,
            DefinitionSource::UserClaude,
            home.join(".claude").join(leaf)
        );
    }

    roots
}

fn push_uniq_root(
    roots: &mut Vec<(DefinitionSource, PathBuf)>,
    source: DefinitionSource,
    path: PathBuf,
){
    if path.is_dir() && !roots.iter().any(|(_, existing)| existing == &path) {
        roots.push((source, path));
    }
}

fn load_agents_from_roots(
    roots: &[(DefinitionSource, PathBuf)]
) -> std::io::Result<Vec<AgentSummary>> {
    let mut agents = Vec::new();
    let mut active_sources = BTreeMap::<String, DefinitionSource>::new();

    for (source, root) in roots {
        let mut root_agents = Vec::new();
        for entry in fs::read_dir(root)? {
            let entry = entry?;
            if entry.path().extension().is_none_or(|ext| ext != "toml") {
                continue;
            }

            let contents = fs::read_to_string(entry.path())?;
            let fallback_name = entry.path().file_stem().map_or_else(
                || entry.file_name().to_string_lossy().to_string(),
                |stem| stem.to_string_lossy().to_string(),
            );

            root_agents.push(AgentSummary {
                name: parse_toml_string(&contents, "name").unwrap_or(fallback_name),
                description: parse_toml_string(&contents, "description"),
                model: parse_toml_string(&contents, "model"),
                reasoning_effort: parse_toml_string(&contents, "model_reasoning_effort"),
                source: *source,
                shadowed_by: None,
            });
        }

        root_agents.sort_by(|left, right| left.name.cmp(&right.name));

        for mut agent in root_agents {
            let key = agent.name.to_ascii_lowercase();
            if let Some(existing) = active_sources.get(&key) {
                agent.shadowed_by = Some(*existing);
            }else {
                active_sources.insert(key, agent.source);
            }
            agents.push(agent);
        }
    }

    Ok(agents)
}

fn parse_toml_string(contents: &str, key: &str) -> Option<String> {
    let prefix = format!("{key} = ");
    for line in contents.lines() {
        let trimmed = line.trim();
        if trimmed.starts_with('#') {
            continue;
        }
        let Some(value) = trimmed.strip_prefix(&prefix) else {
            continue;
        };

        let value = value.trim();
        let Some(value) = value
            .strip_prefix('"')
            .and_then(|value| value.strip_prefix('"'))
        else {
            continue;
        };

        if !value.is_empty() {
            return Some(value.to_string());
        }
    }
    None
}

fn render_agents_report_json(cwd: &Path, agents: &[AgentSummary]) -> Value {
    let active = agents
        .iter()
        .filter(|agent|agent.shadowed_by.is_none())
        .count();
    json!({
       "kind" : "agents",
        "action":"list",
        "working_directory": cwd.display().to_string(),
        "count": agents.len(),
        "summary": {
            "total": agents.len(),
            "active": active,
            "shadowed": agents.len().saturating_sub(active),
        },
        "agents": agents.iter().map(agent_summary_json).collect::<Vec<_>>(),
    })
}

fn agent_summary_json(agents: &AgentSummary) -> Value {
    json!({
        "name": &agents.name,
        "description": &agents.description,
        "model": &agents.model,
        "reasoning_effort": &agents.reasoning_effort,
        "source": definition_source_json(&agents.source),
        "active": agents.shadowed_by.is_none(),
        "shadowed_by": agents.shadowed_by.map(definition_source_json),
    })
}

fn definition_source_json(source: &DefinitionSource) -> Value {
   json!({
       "id": definition_source_id(source),
       "label": source.label(),
   })
}

// 对 DefinitionSource 进行分类，返回一个静态字符串作为ID
fn definition_source_id(source: &DefinitionSource) -> &'static str {
    match source {
        DefinitionSource::ProjectClaw
        | DefinitionSource::ProjectCodex
        | DefinitionSource::ProjectClaude => "project_claw",
        DefinitionSource::UserClawConfigHome | DefinitionSource::UserCodexHome => {
            "user_claw_config_home"
        }
        DefinitionSource::UserClaw | DefinitionSource::UserCodex | DefinitionSource::UserClaude => {
            "user_claw"
        }
    }
}

fn render_agents_usage_json(unexpected: Option<&str>) -> Value {
    json!({
        "kind": "agents",
        "action": "help",
        "usage": {
            "slash_command": "/agents [list|help]",
            "direct_cli": "claw agents [list|help]",
            "sources": ["./claw/agents", "~/.claw/agents", "$CLAW_CONFIG_HOME/agents"]
        },
        "unexpected": unexpected,
    })
}


fn normalize_optional_args(args: Option<&str>) -> Option<&str> {
    args.map(str::trim).filter(|value|{!value.is_empty()})
}

/*
    args = "agents list --help"
    parts = ["agents", "list", "--help"]
    help_index = 2
    返回 Some(vec!["agents", "list"])
 */
fn help_path_from_args(args: &str) -> Option<Vec<&str>> {
    let parts = args.split_whitespace().collect::<Vec<_>>();
    let help_index = parts.iter().position(|part|{is_help_arg(part)})?;
    Some(parts[..help_index].to_vec())
}

fn is_help_arg(arg: &str) -> bool {
    matches!(arg, "help"| "-h" | "--help")
}