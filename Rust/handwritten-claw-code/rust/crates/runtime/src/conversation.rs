use crate::permissions::PermissionPolicy;

pub struct ConversationRuntime<C, T>{
   session: Session,
   api_client: C,
   tool_executor: T,
   permission_policy: PermissionPolicy,
   system_prompt: Vec<String>,
   max_iterations: usize,
   usage_tracker: UsageTracker,
   hook_runner: HookRunner,
   auto_compaction_input_tokens_threshold: u32,
   hook_abort_signal: HookAbortSignal,
   hook_progress_reporter: Option<Box<dyn HookProgressReporter>>,
   session_tracer: Option<SessionTracer>,
}