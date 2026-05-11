#[derive(Debug, Clone, PartialEq)]
pub struct PermissionEnforcer {
    policy: PermissionPolicy,
}
