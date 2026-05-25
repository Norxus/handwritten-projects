use std::collections::BTreeMap;
use std::{env, fs};
use std::path::{Path, PathBuf};
use serde_json::{json, Value};
use runtime::ConfigLoader;

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

pub fn handle_agents_slash_command_json(args: Option<&str>, cwd: &Path) -> std::io::Result<Value> {
    if let Some(args) = normalize_optional_args(args) {
        if let Some(help_path) = help_path_from_args(args) {
            return Ok(
                match help_path.as_slice() {
                    [] => render_agents_usage_json(None),
                    _  => render_agents_usage_json(Some(&help_path.join(" "))),
                }
            );
        }
    }

    match normalize_optional_args(args) {
        None | Some("list") => {
            let roots = discover_definition_roots(cwd, "agents");
            let agents = load_agents_from_roots(&roots)?;
            Ok(render_agents_report_json(cwd,&agents))
        }
        Some(args) if is_help_arg(args) => Ok(render_agents_usage_json(None)),
        Some(args) => Ok(render_agents_usage_json(Some(args))),
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

pub fn handle_mcp_slash_command(
    args: Option<&str>,
    cwd: &Path,
) -> Result<Value, runtime::ConfigError> {
    let loader = ConfigLoader::default_for(cwd);
    render_mcp_report_json_for(&loader, cwd, args)
}

pub fn handle_mcp_slash_command_json(
    args: Option<&str>,
    cwd: &Path,
) -> Result<Value, runtime::ConfigError> {
    let loader = ConfigLoader::default_for(cwd);
    render_mcp_report_json_for(&loader, cwd, args)
}

pub fn handle_skills_slash_command(args: Option<&str>, cwd: &Path) -> std::io::Result<String> {
    if let Some(args) = normalize_optional_args(args) {
        if let Some(help_path) = help_path_from_args(args) {
            return Ok(match help_path.as_slice() {
                [] => render_skills_usage(None),
                ["install", ..] => render_skills_usage(Some("install")),
                _ => render_skills_usage(Some(&help_path.join(" "))),
            });
        }
    }

    match normalize_optional_args(args) {
        None | Some("list") => {
            let roots = discover_skill_roots(cwd);
            let skills = load_skills_from_roots(&roots)?;
            Ok(render_skills_report(&skills))
        }
        Some(args) if args.starts_with("list ") => {
            // 取出 list 后面的查询词
            let filter = args["list ".len()..].trim().to_lowercase();
            let roots = discover_skill_roots(cwd);
            let skills = load_skills_from_roots(&roots)?;
            let filtered: Vec<_> = skills
                .into_iter()
                .filter(|s| s.name.to_lowercase().contains(&filter))
                .collect();
            Ok(render_skills_report(&filtered))
        }
        Some("show" | "info" | "describe") => {
            let roots = discover_skill_roots(cwd);
            let skills = load_skills_from_roots(&roots)?;
            Ok(render_skills_report(&skills))
        }
        // 用来处理 show <name> 、 info <name> 、 describe <name> 这三类“按技能名查看技能信息
        Some(args)
        if args.starts_with("show")
            || args.starts_with("info")
            || args.starts_with("describe") => {
            let name = args.splitn(2, ' ')
                .nth(1)
                .unwrap_or_default()
                .trim()
                .to_lowercase();
            let roots = discover_skill_roots(cwd);
            let skills = load_skills_from_roots(&roots)?;
            // 匹配所有的 skills ，找到要求的 skill
            let matched: Vec<_> = skills.into_iter()
                .filter(|s| s.name.to_lowercase() == name)
                .collect();
            Ok(render_skills_report(&matched))
        }
        // => { ... } ：逗号可以省略，这里不能省略
        Some("install") => Ok(render_skills_usage(Some("install"))),
        Some(args) if args.unwrap().starts_with("install ") => {
            let target = args["install ".len()..].trim();
            // 如果用户没有指定 install 什么东西
            if target.is_empty() {
                return Ok(render_skills_usage(Some("install")));
            }
            let install = install_skill(target, cwd)?;
            Ok(render_skill_install_report(&install))
        }
        Some(args) if is_help_arg(args)  => Ok(render_skills_usage(None)),
        // 没办法处理这个参数，最后的 fallback 分支
        Some(args) => Ok(render_skills_usage(Some(args)))
    }
}

pub fn handle_skills_slash_command_json(args: Option<&str>, cwd: &Path) -> std::io::Result<Value> {
    if let Some(args) = normalize_optional_args(args) {
        if let Some(help_path) = help_path_from_args(args) {
            return Ok(match help_path.as_slice() {
                [] => render_skills_usage_json(None),
                ["install", ..] => render_skills_usage_json(Some("install")),
                _ => render_skills_usage_json(Some(&help_path.join(" "))),
            });
        }
    }

    match normalize_optional_args(args) {
        None | Some("list") => {
            let roots = discover_skill_roots(cwd);
            let skills = load_skills_from_roots(&roots)?;
            Ok(render_skills_report_json(&skills))
        }
        Some(args) if args.starts_with("list ") => {
            let filter = args["list ".len()..].trim().to_lowercase();
            let roots = discover_skill_roots(cwd);
            let skills = load_skills_from_roots(&roots)?;
            let filtered: Vec<_> = skills
                .into_iter()
                .filter(|s| s.name.to_lowercase().contains(&filter))
                .collect();
            Ok(render_skills_report_json(&filtered))
        }
        Some("show" | "info" | "describe") => {
            let roots = discover_skill_roots(cwd);
            let skills = load_skills_from_roots(&roots)?;
            Ok(render_skills_report_json(&skills))
        }
        Some(args)
            if args.starts_with("show ")
                || args.starts_with("info ")
                || args.starts_with("describe ") =>
        {
            let name = args
                .splitn(2, ' ')
                .nth(1)
                .unwrap_or_default()
                .trim()
                .to_lowercase();
            let roots = discover_skill_roots(cwd);
            let skills = load_skills_from_roots(&roots)?;
            let matched: Vec<_> = skills
                .into_iter()
                .filter(|s| s.name.to_lowercase() == name)
                .collect();
            Ok(render_skills_report_json(&matched))
        }
        Some("install") => Ok(render_skills_usage_json(Some("install"))),
        Some(args) if args.starts_with("install ") => {
            let target = args["install ".len()..].trim();
            if target.is_empty() {
                return Ok(render_skills_usage_json(Some("install")));
            }
            let install = install_skill(target, cwd)?;
            Ok(render_skill_install_report_json(&install))
        }
        Some(args) if is_help_arg(args) => Ok(render_skills_usage_json(None)),
        Some(args) => Ok(render_skills_usage_json(Some(args))),
    }
    
}

fn render_skill_install_report_json(skill: &InstalledSkill) -> Value {
    json!({
        "kind": "skills",
        "action": "install",
        "result": "installed",
        "invocation_name": &skill.invocation_name,
        "invoke_as": format!("${}", skill.invocation_name),
        "display_name": &skill.display_name,
        "source": skill.source.display().to_string(),
        "registry_root": skill.registry_root.display().to_string(),
        "installed_path": skill.installed_path.display().to_string(),
    })
}

fn render_skills_report_json(skills: &[SkillSummary]) -> Value {
    let active = skills.iter()
        .filter(|skill|skill.shadowed_by.is_none())
        .count();

    json!({
        "kind": "skills",
        "action": "list",
        "summary": {
            "total": skills.len(),
            "active": active,
            "shadowed": skills.len().saturating_sub(active),
        },
        "skills": skills.iter().map(skill_summary_json).collect::<Vec<_>>(),
    })
}

fn skill_summary_json(skill: &SkillSummary) -> Value {
    json!({
        "name": &skill.name,
        "description": &skill.description,
        "source": definition_source_json(skill.source),
        "origin": skill_origin_json(skill.origin),
        "active": skill.shadowed_by.is_none(),
        "shadowed_by": skill.shadowed_by.map(definition_source_json),
    })
}

fn skill_origin_json(origin: SkillOrigin) -> Value {
    json!({
        "id": skill_origin_id(origin),
        "detail_label": origin.detail_label(),
    })
}

fn skill_origin_id(origin: SkillOrigin) -> &'static str {
    match origin {
        SkillOrigin::SkillsDir => "skills_dir",
        SkillOrigin::LegacyCommandsDir => "legacy_commands_dir",
    }
}

fn render_skills_usage_json(unexpected: Option<&str>) -> Value {
    json!({
        "kind": "skills",
        "action": "help",
        "usage": {
            "slash_command": "/skills [list|install <path>|help|<skill> [args]]",
            "aliases": ["/skill"],
            "direct_cli": "claw skills [list|install <path>|help|<skill> [args]]",
            "invoke": "/skills help overview -> $help overview",
            "install_root": "$CLAW_CONFIG_HOME/skills or ~/.claw/skills",
            "sources": [
                ".claw/skills",
                ".omc/skills",
                ".agents/skills",
                ".codex/skills",
                ".claude/skills",
                "~/.claw/skills",
                "~/.omc/skills",
                "~/.claude/skills/omc-learned",
                "~/.codex/skills",
                "~/.claude/skills",
                "legacy /commands",
                "legacy fallback dirs still load automatically"
            ],
        },
        "unexpected": unexpected,
    })
}


fn render_skill_install_report(skill: &InstalledSkill) -> String {
    let mut lines = vec![
        "Skills".to_string(),
        format!("  Result           installed {}", skill.invocation_name),
        format!("  Invoke as        ${}", skill.invocation_name),
    ];

    if let Some(display_name) = &skill.display_name {
        lines.push(format!("  Display name     {display_name}"));
    }
    lines.push(format!("  Source           {}", skill.source.display()));
    lines.push(format!(
        "  Registry         {}",
        skill.registry_root.display()
    ));
    lines.push(format!(
        "  Installed path   {}",
        skill.installed_path.display()
    ));
    lines.join("\n")
}

fn install_skill(source: &str, cwd: &Path) -> std::io::Result<InstalledSkill> {
    let registry_root = default_skill_registry_root(cwd)?;
    install_skill_into(source, cwd, &registry_root)
}

fn default_skill_install_root() -> std::io::Result<PathBuf> {
    if let Ok(claw_config_home)  = env::var("CLAW_CONFIG_HOME") {
        return Ok(PathBuf::from(claw_config_home).join("skills"));
    }
    if let Ok(codex_home) = env::var("CODEX_HOME") {
        return Ok(PathBuf::from(codex_home).join("skills"));
    }
    if let Some(home) = env::var_os("HOME") {
        return Ok(PathBuf::from(home).join(".claw").join("skills"));
    }
    Err(std::io::Error::new(
        std::io::ErrorKind::NotFound,
        "unable to resolve a skills install root; set CLAW_CONFIG_HOME or HOME",
    ))
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct InstalledSkill {
    invocation_name: String,
    display_name: Option<String>,
    source: PathBuf,
    registry_root: PathBuf,
    installed_path: PathBuf,
}

// 把一个 skill 源文件/目录（source）安装（也就是复制）到本地 skill registry 里，并返回安装后的元信息
// cwd 用来把用户传入的相对路径 source 解析成绝对路径
fn install_skill_into(
    source: &str,
    cwd: &Path,
    registry_root: &Path,
) -> std::io::Result<InstalledSkill> {
    let source = resolve_skill_install_source(source, cwd)?;
    let prompt_path = source.prompt_path();
    let contents = fs::read_to_string(prompt_path)?;
    // md 文件里的名字
    let display_name = parse_skill_frontmatter(&contents).0;
    // 将 display_name 按照规则进行清洗，如果没有使用 source.fallback_name()
    let invocation_name = derive_skill_install_name(&source, display_name.as_deref())?;
    // 拼接完整的下载路径
    let installed_path  = registry_root.join(&invocation_name);

    if installed_path.exists() {
        return Err(std::io::Err::new(
            std::io::ErrorKind::AlreadyExists,
            format!(
                "skill '{invovation_name}' is already install at {}",
                installed_path.display()
            ),
        ));
    }

    // 创建安装目录
    fs::create_dir_all(&installed_path)?;
    let install_result = match &source {
        SkillInstallSource::Directory { root, ..} => {
            // 如果 source 是目录，那么复制整个目录 root -> installed_path
            copy_directory_contents(root, &installed_path)
        }
        SkillInstallSource::MarkdownFile { path } => {
            // 如果 source 是文件，那么复制该文件到 installed_path/SKILL.md
            fs::copy(path, installed_path.join("SKILL.md")).map(|_|())
        }
    };

    // 如果有错误，把之前创建好的文件夹删除掉
    if let Err(error) = install_result {
        let _ = fs::remove_dir_all(&installed_path);
        return Err(error);
    }

    Ok(
        InstalledSkill {
            invocation_name,
            display_name,
            source: source.report_path().to_path_buf(),
            registry_root: registry_root.to_path_buf(),
            installed_path,
        }
    )

    

}

fn derive_skill_install_name(
    source: &SkillInstallSource,
    declared_name: Option<&str>
) -> std::io::Result<String>{
    for candidate in [declared_name, source.fallback_name().as_deref()]{
        // 按照规则清洗 candidate, 只要有一个候选名清洗后有效
        if let Some(candidate) = candidate.and_then(sanitize_skill_invocation_name) {
            return Ok(candidate)
        }
    }

     Err(std::io::Error::new(
        std::io::ErrorKind::InvalidInput,
        format!(
            "unable to derive an installable invocation name from '{}'",
            source.report_path().display()
        ),
    ))
}

// 把一个原始名字 candidate 清洗成一个“可安装/可调用”的规范名字
fn sanitize_skill_invocation_name(candidate: &str) -> Option<String> {
    let trimmed = candidate.trim().trim_start_matches('/').trim_start_matches('$');
    if trimmed.is_empty() {
        return None
    }

    let mut sanitized = String::new();
    let mut last_was_separator = false;

    for ch in trimmed.chars() {
        if ch.is_ascii_alphanumeric() || matches!(ch, '-'|'_'|'.') {
            sanitized.push(ch.to_ascii_lowercase());
            last_was_separator = false
        }else if (ch.is_whitespace() || matches!(ch, '/' | '\\')) && !last_was_separator && !sanitized.is_empty() {
            sanitized.push('-');
            last_was_separator = true;
        }
    }


    let sanitized = sanitized.trim_matches(|ch| matches!(ch, '-'|'_'|'.')).to_string();
    (!sanitized.is_empty()).then_some(sanitized)
}

enum SkillInstallSource {
    Directory{root: PathBuf, prompt_path: PathBuf},
    MarkdownFile{path: PathBuf},
}

impl SkillInstallSource {
    fn prompt_path(&self) -> &Path {
        // 这里 match 的 self 是一个引用，所以解构的结果也是引用，所以是 &PathBuf 类型
        // 返回时，编译器做了强转，&PathBuf -> &Path
        match self {
            Self::Directory{prompt_path, ..} => prompt_path,
            Self::MarkdownFile {path} => path,
        }
    }

    // 当技能文件里没有显式声明 name 时，就从路径里推一个名字出来。
    fn fallback_name(&self) -> Option<String> {
        match self {
            // 对于目录来源取目录名
            Self::Directory { root, ..} => root.file_name().map(|name|name.to_string_lossy().to_string()),
            // 对于 markdown 文件来源，取文件名去掉拓展名后的 stem
            Self::MarkdownFile { path } => path.file_stem().map(|name|name.to_string_lossy().to_string()),
        }
    }
}

// 找出 skill 源的真正路径，统一成 SkillInstallSource 类型
fn resolve_skill_install_source(source: &str, cwd: &Path) -> std::io::Result<SkillInstallSource> {
    let candidate = PathBuf::from(source);
    let source = if candidate.is_absolute() {
        candidate
    }else {
        cwd.join(candidate)
    };
    // 得到一个“规范化后的绝对路径”，包括：
    // 把相对路径变成绝对路径
    // - 解析 . 和 ..
    // - 跟随符号链接，拿到真实路径
    let source = fs::canonicalize(&source)?;

    // 如果是目录，检查目录下是否有 SKILL.md 文件
    if source.is_dir() {
        let prompt_path = source.join("SKILL.md");
        if !prompt_path.is_file() {
            return Err(std::io::Error::new(
                std::io::ErrorKind::InvalidInput,
                format!(
                    "skill directory '{}' must contain SKILL.md",
                    source.display()
                )
            ));
        }
        return Ok(SkillInstallSource::Directory {
            root: source,
            prompt_path,
        });
    }

    // 如果不是目录，检查是否是 markdown 文件
    if source.extension()
        .is_some_and(|ext| ext.to_string_lossy().eq_ignore_ascii_case("md")){
        return Ok(SkillInstallSource::MarkdownFile{path: source});
    }

    Err(std::io::Error::new(
        std::io::ErrorKind::InvalidInput,
        format!(
            "skill source '{}' must be a directory with SKILL.md or a markdown file",
            source.display()
        ),
    ))
}

#[allow(clippy::too_many_lines)]
fn render_skills_usage(unexpected: Option<&str>) -> String {
    let mut lines = vec![
        "Skills".to_string(),
        "  Usage            /skills [list|install <path>|help|<skill> [args]]".to_string(),
        "  Alias            /skill".to_string(),
        "  Direct CLI       claw skills [list|install <path>|help|<skill> [args]]".to_string(),
        "  Invoke           /skills help overview -> $help overview".to_string(),
        "  Install root     $CLAW_CONFIG_HOME/skills or ~/.claw/skills".to_string(),
        "  Sources          .claw/skills, .omc/skills, .agents/skills, .codex/skills, .claude/skills, ~/.claw/skills, ~/.omc/skills, ~/.claude/skills/omc-learned, ~/.codex/skills, ~/.claude/skills, legacy /commands".to_string(),
    ];
    if let Some(args) = unexpected {
        lines.push(format!("  Unexpected: {}", args));
    }
    lines.join("\n")
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct SkillRoot {
    source: DefinitionSource,
    path: PathBuf,
    origin: SkillOrigin,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum SkillOrigin {
    SkillsDir,
    LegacyCommandsDir,
}

impl SkillOrigin {
    fn detail_label(self) -> Option<&'static str> {
        match self {
            Self::SkillsDir => None,
            Self::LegacyCommandsDir => Some("legacy /commands"),
        }
    }
}

fn discover_skill_roots(cwd: &Path) -> Vec<SkillRoot> {
    let mut roots = Vec::new();

    for ancestor in cwd.ancestors() {
        push_unique_skill_root(
            &mut roots,
            DefinitionSource::ProjectClaw,
            ancestor.join(".claw").join("skills"),
            SkillOrigin::SkillsDir,
        );
        push_unique_skill_root(
            &mut roots,
            DefinitionSource::ProjectClaw,
            ancestor.join(".omc").join("skills"),
            SkillOrigin::SkillsDir,
        );
        push_unique_skill_root(
            &mut roots,
            DefinitionSource::ProjectClaw,
            ancestor.join(".agents").join("skills"),
            SkillOrigin::SkillsDir,
        );
        push_unique_skill_root(
            &mut roots,
            DefinitionSource::ProjectCodex,
            ancestor.join(".codex").join("skills"),
            SkillOrigin::SkillsDir,
        );
        push_unique_skill_root(
            &mut roots,
            DefinitionSource::ProjectClaude,
            ancestor.join(".claude").join("skills"),
            SkillOrigin::SkillsDir,
        );
        push_unique_skill_root(
            &mut roots,
            DefinitionSource::ProjectClaw,
            ancestor.join(".claw").join("commands"),
            SkillOrigin::LegacyCommandsDir,
        );
        push_unique_skill_root(
            &mut roots,
            DefinitionSource::ProjectCodex,
            ancestor.join(".codex").join("commands"),
            SkillOrigin::LegacyCommandsDir,
        );
        push_unique_skill_root(
            &mut roots,
            DefinitionSource::ProjectClaude,
            ancestor.join(".claude").join("commands"),
            SkillOrigin::LegacyCommandsDir,
        );
    }

    if let Ok(claude_config_dir) = env::var("CLAUDE_CONFIG_DIR") {
        let claude_config_dir = PathBuf::from(claude_config_dir);
        let skills_dir = claude_config_dir.join("skills");
        push_unique_skill_root(
            &mut roots,
            DefinitionSource::UserClaude,
            skills_dir.clone(),
            SkillOrigin::SkillsDir,
        );
        push_unique_skill_root(
            &mut roots,
            DefinitionSource::UserClaude,
            skills_dir.join("omc-learned"),
            SkillOrigin::SkillsDir,
        );
        push_unique_skill_root(
            &mut roots,
            DefinitionSource::UserClaude,
            claude_config_dir.join("commands"),
            SkillOrigin::LegacyCommandsDir,
        );
    }

    roots
}

// 仅添加目录，且去重
fn push_unique_skill_root(
    roots: &mut Vec<SkillRoot>,
    source: DefinitionSource,
    path: PathBuf,
    origin: SkillOrigin,
) {
    // 路径是目录，且不是已存在的路径
    if path.is_dir() &&!roots.iter().any(|existing|existing.path == path) {
        roots.push(SkillRoot {
            source,
            path,
            origin,
        });
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct SkillSummary {
    name: String,
    description: Option<String>,
    source: DefinitionSource,
    shadowed_by: Option<DefinitionSource>,
    origin: SkillOrigin,
}

fn load_skills_from_roots(roots: &[SkillRoot]) -> std::io::Result<Vec<SkillSummary>> {
    let mut skills = Vec::new();
    let mut active_sources = BTreeMap::<String, DefinitionSource>::new();
    for root in roots {
        let mut root_skills = Vec::new();
        for entry in fs::read_dir(&root.path)? {
            // 通过解构取值，然后 shadowing
            let entry = entry?;
            match root.origin {
                SkillOrigin::SkillsDir => {
                    if !entry.path().is_dir() {
                        continue;
                    }
                    // 读取该路径下的 SKILL.md 文件
                    let skill_path = entry.path().join("SKILL.md");
                    if !skill_path.is_file() {
                       continue;
                    }
                    let contents = fs::read_to_string(&skill_path)?;
                    let (name, description) = parse_skill_frontmatter(&contents)?;
                    root_skills.push(SkillSummary {
                        name: name
                            // to_string_lossy 把“可能不是合法 UTF-8 的字节/系统字符串”尽量转换成可显示的字符串
                            .unwrap_or_else(|| entry.file_name().to_string_lossy().to_string()),
                        description,
                        source: root.source,
                        shadowed_by: None,
                        origin: root.origin,
                    })
                }
                // 兼容老版本的 Skill 目录
                SkillOrigin::LegacyCommandsDir => {
                    let path  = entry.path();
                    let markdown_path = if path.is_dir() {
                        let skill_path = path.join("SKILL.md");
                        if !skill_path.is_file() {
                            continue;
                        }
                        skill_path
                    }else if path.extension()
                        .is_some_and(|ext| ext.to_string_lossy().eq_ignore_ascii_case("md")) {
                        path
                    }else {
                        continue;
                    };

                    let contents = fs::read_to_string(&markdown_path)?;
                    // markdown_path.file_stem() 取文件名去掉扩展名后的部分
                    // entry.file_name() 取这个目录项最后一段名字，保留扩展名，用于最后的
                    let fallback_name = markdown_path.file_stem().map_or_else(
                        || entry.file_name().to_string_lossy().to_string(),
                        |stem| stem.to_string_lossy().to_string(),
                    );
                    let (name, description) = parse_skill_frontmatter(&contents)?;
                    root_skills.push(SkillSummary {
                        name: name.unwrap_or(fallback_name),
                        description,
                        source: root.source,
                        shadowed_by: None,
                        origin: root.origin,
                    })
                }
            }

        }

        // 两个地方的 skill 进行排序
        root_skills.sort_by(|left, right|left.name.cmp(&right.name));

        for mut skill in root_skills {
            let key = skill.name.to_ascii_lowercase();
            // 查询不需要所有权
            if let Some(existing) = active_sources.get(&key) {
                skill.shadowed_by = Some(*existing);
            }else {
                // insert 需要所有权
                active_sources.insert(key, skill.source);
            }
            skills.push(skill);
        }
    }
    Ok(skills)
}


fn render_skills_report(skills: &[SkillSummary]) -> String {
    if skills.is_empty() {
        return "No skills found".to_string();
    }

    let total_active = skills
        .iter()
        .filter(|skill| skill.shadowed_by.is_none())
        .count();

    let mut lines = vec![
        "Skills".to_string(),
        format!(" {total_active} available skills"),
        String::new(),
    ];

    for scope in [
        DefinitionScope::Project,
        DefinitionScope::UserConfigHome,
        DefinitionScope::UserHome,
    ] {
        let group = skills
            .iter()
            .filter(|skill| skill.source.report_scope() == scope)
            .collect::<Vec<_>>();
        if group.is_empty() {
            continue;
        }

        lines.push(format!("{}:", scope.label()));
        for skill in group {
            let mut parts = vec![skill.name.clone()];
            if let Some(description) = &skill.description {
                parts.push(description.clone());
            }
            if let Some(detail) = skill.origin.detail_label() {
                parts.push(detail.to_string());
            }
            let detail = parts.join(" · ");
            match skill.shadowed_by {
                None => lines.push(detail),
                Some(winner) => lines.push(format!("(shadowed by {}) {detail}", winner.label())),
            }
        }
        lines.push(String::new());
    }
    lines.join("\n").trim_end().to_string()
}

// 解析如下格式：
// ---
// name: "foo"
// description: 'bar'
// ---
// 正文内容
fn parse_skill_frontmatter(contents: &str) -> (Option<String>, Option<String>) {
    let mut lines = contents.lines();
    // 第一行在这里被消费掉了，后面再迭代的时候会跳过这行
    if lines.next().map(str::trim) != Some("---") {
        return (None, None);
    }

    let mut name = None;
    let mut description = None;
    for line in lines {
        let trimmed = line.trim();
        if trimmed == "---"  {
            break;
        }

        if let Some(value) = trimmed.strip_prefix("name:") {
            let value = unquote_frontmatter_value(value.trim());
            if !value.is_empty() {
                name = Some(value);
            }
            continue;
        }

        if let Some(value) = trimmed.strip_prefix("description:") {
            let value = unquote_frontmatter_value(value.trim());
            if !value.is_empty() {
                description = Some(value);
            }
            continue;
        }
    }
    (name, description)
}

// 1. 优先尝试去掉最外层双引号
// 2. 不行的话再尝试去掉最外层单引号
// 3. 都不行就保留原始值
// 4. 最后去掉首尾空白并转成 String
fn unquote_frontmatter_value(value: &str) -> String {
    value
        .strip_prefix('"')
        .and_then(|trimmed| trimmed.strip_suffix('"'))
        .or_else(|| {
            value.strip_prefix('\'')
                .and_then(|trimmed| trimmed.strip_suffix('\''))
        })
        .unwrap_or(value)
        .trim()
        .to_string()
}



fn render_mcp_report_json_for(
    loader: &ConfigLoader,
    cwd: &Path,
    args: Option<&str>,
) -> Result<Value, runtime::ConfigError> {
    if let Some(args) = normalize_optional_args(args) {
        if let Some(help_path) = help_path_from_args(args) {
            return Ok(match help_path.as_slice() {
                // 说明命令是 /mcp help
                [] => render_mcp_usage_json(None),
                // 说明命令是 /mcp show ... help，["show,...] 是切片模式匹配，表示第一个元素必须是 "show"
                ["show", ..] => render_mcp_usage_json(Some("show")),
                _ => render_mcp_usage_json(Some(&help_path.join(" "))),
            });
        }
    }

    match normalize_optional_args(args) {
        None | Some("list") => {
            match loader.load() {
                Ok(runtime_config) => {
                    let mut value = render_mcp_summary_report_json(cwd, runtime_config.mcp().servers());
                    if let Some(map) = value.as_object_mut() {
                        map.insert("status".to_string(), Value::String("ok".to_string()));
                        map.insert("config_load_error".to_string(), Value::Null);
                    }
                    Ok(value)
                }
            }
        }
    }
}

fn render_mcp_summary_report_json(
    cwd: &Path,
    servers: &BTreeMap<String, ScopedMcpServerConfig>) -> Value {
    json!({
        "kind": "mcp",
        "action": "list",
        "working_directory": cwd.display().to_string(),
        "configured_servers": servers.len(),
        "servers": servers
        .iter()
        .map(|(name, server)| mcp_server_json(name, server))  \
        .collect::<Vec<_>>(),
    })
}

fn render_mcp_usage_json(unexpected: Option<&str>) -> Value {
    json!({
        "kind": "mcp",
        "action": "help",
        "usage": {
            "slash_command": "/mcp [list|show <server> | help]",
            "direct_cli": "claw mcp [list|show <server> | help]",
            "source": [".claw/settings.json", ".claw/settings.local.json"]
        },
        "unexpected": unexpected,
    })
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

fn definition_source_json(source: DefinitionSource) -> Value {
   json!({
       "id": definition_source_id(source),
       "label": source.label(),
   })
}

// 对 DefinitionSource 进行分类，返回一个静态字符串作为ID
fn definition_source_id(source: DefinitionSource) -> &'static str {
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
