import logging
from copy import deepcopy
from datetime import datetime, timezone
from optparse import Option
from typing import Any, Dict, Optional

from mem0.exceptions import ValidationError as Mem0ValidationError
from mem0.memory.base import MemoryBase
from mem0.memory.setup import setup_config

logger = logging.getLogger(__name__)

_RUNTIME_FIELDS = frozenset({"http_auth", "auth", "connection_class", "ssl_context"})

_SENSITIVE_FIELDS_EXTRACT = frozenset(
    {
        "api_key",
        "secret_key",
        "private_key",
        "access_key",
        "password",
        "credentials",
        "credential",
        "secret",
        "token",
        "access_token",
        "client_secret",
        "auth_clent_secret",
        "azure_client_secret",
        "service_account_json",
        "aws_session_token",
    }
)

_SENSITIVE_SUFFIXES = ("_password", "_secret", "_token", "_credential", "_credentials")

ENTITY_PARAMS = frozenset({"user_id", "agent_id", "run_id"})


# 检查传入的 kwargs 里，是否包含不允许放在顶层的实体参数
# 强制调用方把这些参数放进 filters 里，而不是直接写成函数参数
def _reject_top_level_entity_params(kwargs: Dict[str, Any], method_name: str) -> None:
    """Reject top-level entity patamters - must use filters instead.l"""
    invalid_keys = ENTITY_PARAMS & set(kwargs.keys())
    if invalid_keys:
        raise ValueError(
            f"""Top-level entity parameters {invalid_keys}
                are not supported in {method_name}()."""
            f"""Use filters={{'user_id': '...'}} instead."""
        )


# 把传进来的 ID 做一次清洗和合法性检查
def _validate_and_trim_entity_id(value: Optional[str], name: str) -> Optional[str]:
    """
    Validates and normalizes an entity ID.
    - Trims leading/trailing whitespace
    - Reject empty or whitespace-only strings
    - Rejects strings containing internal whitespace

    Args:
        value: The entity ID value to validate
        name: The parameter name (for error message)

    Returns:
        the trimmed entity ID. or None if input is None

    Raises:
        ValueError: If entityID is invalid
    """
    if value is None:
        return None

    trimmed = value.strip()
    if trimmed == "":
        raise ValueError(f"Invalid {name}: cannot be empty or whitespace-only. Provide a valid indentifier.")

    if any(c.isspace() for c in trimmed):
        raise ValueError(f"Invalid {name}: cannot contain whitspace. Provide a valid identifier without space.")
    return trimmed


# 验证搜索参数


def _validate_search_params(threshold: Optional[float] = None, top_k: Optional[int] = None) -> None:
    """
    Validates search parameters.

    Args:
        threshold: Similarity threshold (must be between 0 and 1)
        top_k: Number of result to return (must be non-negative integer)

    Raises:
        ValueError: If threshold or top_k are invalid
    """
    if threshold is not None:
        if not isinstance(threshold, (int, float)):
            raise ValueError("threshold must be a valid number")
        if threshold < 0 or threshold > 1:
            raise ValueError(f"Invalid threshold: {threshold}. Must be between 0 and 1 (inclusive)")
    if top_k is not None:
        # 在 Python 里， bool 是 int 的子类，isinstance(True, int) 会返回 True
        if not isinstance(top_k, int) or isinstance(top_k, bool):
            raise ValueError("top_k must be a valid integer")
        if top_k < 0:
            raise ValueError(f"Invalid top_k: {top_k}. Must be a non-negative integer.")


#  判断是否是敏感列


def _is_sensitive_field(field_name: str) -> bool:
    """check if a field should be redacted for telemetry safety.

    Uses a layered approach:
    1. Runtime fields(allowlist) - always preserved, highest priority.
    2. Exact deny list - known secret field names.
    3. Suffix deny list - caches patterns like db_password, auth_secret, etc.
    """
    name = field_name.lower().strip()
    if name in _RUNTIME_FIELDS:
        return False
    if name in _SENSITIVE_FIELDS_EXTRACT:
        return True
    return any(name.endswith(suffix) for suffix in _SENSITIVE_SUFFIXES)


# 想尽办法 deepcopy config，内置多种容错手段


def _safe_deepcopy_config(config):
    """Safely deepcopy config, failing back to dict-based cloning for non-serializable objects."""
    try:
        return deepcopy(config)
    except Exception as e:
        logger.debug(f"Deepcopy failed, using dict-based clonig: {e}")

        # 获取 config 的类
        config_class = type(config)

        # 如果有 model_dump 函数，就直接调用
        if hasattr(config, "model_dump"):
            try:
                clone_dict = config.model_dump()
            except Exception:
                clone_dict = dict(config.__dict__)
        else:
            # 不然浅拷贝最外层 kv 字段
            clone_dict = dict(config.__dict__)

        # 遍历导出的字段字典，逐个判断要不要修正
        for field_name in list(clone_dict.keys()):
            # 可能来自 config.model_dump()，model_dump 可能会丢掉这些运行时属性
            if field_name in _RUNTIME_FIELDS and hasattr(config, field_name):
                clone_dict[field_name] = getattr(config, field_name)
            # 敏感字段直接屏蔽
            elif _is_sensitive_field(field_name):
                clone_dict[field_name] = None

        try:
            # 尝试用原类构造一个新实例，解包字典后调用 init 函数
            return config_class(**clone_dict)
        except Exception:
            logger.debug("Config reconstruction failed, returning shallow dict clone")
            # 动态创建了一个临时类，类名 Config ，() 表示不继承任何父类，实际上默认继承 object
            # 把 clone_dict 的键值作为这个临时类的属性挂上去，然后实例化返回
            return type("Config", (), clone_dict)()


def _normalize_iso_timestamp_to_utc(timestamp: Optional[str]) -> Optional[str]:
    """Normalize timezone-aware ISO timestamps to UTC without rewriting naive values."""
    if not timestamp:
        return timestamp

    try:
        # 要求输入大致符合 ISO 8601 格式，比如 2026-06-22T10:00:00+08:00
        parsed = datetime.fromisoformat(timestamp)
    except ValueError:
        return timestamp

    # 如果解析出来的是“无时区时间
    if parsed.tzinfo is None:
        return timestamp

    # 利用 utc 作为中间态
    # 如果时间本身带时区，比如 +08:00 、 -05:00 ，就先转换成 UTC，再转回 ISO 字符串返回
    return parsed.astimezone(timezone.utc).isoformat()


# 把“这次记忆操作属于谁/哪个会话”的信息整理成两份字典，一份给“写入时的 metadata”，一份给“查询时的 filters”
# 其中 actor_id 比较特殊，actor_id 只参与“查询过滤”，不写入“存储 metadata 模板”
def _build_filters_and_metadata(
    *,
    user_id: Optional[str] = None,
    agent_id: Optional[str] = None,
    run_id: Optional[str] = None,
    actor_id: Optional[str] = None,
    input_metadata: Optional[Dict[str, Any]] = None,
    input_filters: Optional[Dict[str, Any]] = None,
) -> tuple[Dict[str, any], Dict[str, Any]]:
    """
    Constructs metadata for storage and filters for querying based on session and actor identifiers.

    This helper supports multiple session identifiers (`user_id`, `agent_id`, and/or `run_id`)
    for flexible session scoping and optionally narrows queriews to specific `actor_id`. It returns two dicts:

    1. `base_metadata_template`: Used as a template for metadata when storing new memories.
        It includes all provided session identifier(s) and any `input_metadata`.
    2. `effective_query_filters`: Used for querying existing memories. It includes all provided session
        identifier(s), and a resolved actor identifier for targeted filtering if specified by any actor-related inputs.

    Actor filtering precedence: explicit `actor_id` arg -> `filters["actor_id"]`
    This resolved actor ID is used for querying but is not added to `base_metadata_template`,
    as the actor for storage is typically derived from message content at a large stage.

    Args:
        user_id (Optional[str]): User identifier, for session scoping.
        agent_id (Optional[str]): Agent identifier, for session scoping.
        run_id (Optional[str]):  Run identifier, for session scoping.
        actor_id (Optional[str]): Explicit actor identifier, used as a pentential source for
            actor-specific filtering. See actor resolution precedence in the main description.
        input_metadata (Optional[Dict[str, Any]]): Base dictionary to be augmented with
            session identifiers for the storage metadata template. Defaults to an empty dict.
        input_filters (Optional[Dict[str, Any]]): Base dictionary to be augmented with
            session and actor identifiers for query filters. Default to an empty dict.

    Returns:
        tuple[Dict[str, Any], Dict[str, Any]]: A tuple containing:
            - base_metadata_template[Dict[str, Any]]: Metadata template for storing memories,
                scoped to the provided session(s).
            - effective_query_filters (Dict[str, Any]): filters for querying memories,
                scoped to the provided session(s) and potentially a resolved actor.
    """
    base_metadata_template = deepcopy(input_metadata) if input_metadata else {}
    effective_query_filters = deepcopy(input_filters) if input_filters else {}

    session_ids_provided = []

    user_id = _validate_and_trim_entity_id(user_id, "user_id")
    agent_id = _validate_and_trim_entity_id(agent_id, "agent_id")
    run_id = _validate_and_trim_entity_id(run_id, "run_id")

    if user_id:
        base_metadata_template["user_id"] = user_id
        effective_query_filters["user_id"] = user_id
        session_ids_provided.append("user_id")

    if agent_id:
        base_metadata_template["agent_id"] = agent_id
        effective_query_filters["agent_id"] = agent_id
        session_ids_provided.append("agent_id")

    if run_id:
        base_metadata_template["run_id"] = run_id
        effective_query_filters["run_id"] = run_id
        session_ids_provided.append("run_id")

    if not session_ids_provided:
        raise Mem0ValidationError(
            message="At least one of 'user_id', 'agent_id', or 'run_id' must be provided.",
            error_code="VALIDATION_001",
            details={"provided_ids": {"user_id": user_id, "agent_id": agent_id, "run_id": run_id}},
            suggestion="Please provide at least one identifier to scope the memory operation.",
        )

    # 真正写入时的 actor，通常由后续消息解析阶段决定，而不是直接照搬当前传参，所以不把 actor_id 写入基础元信息中
    resolved_actor_id = actor_id or effective_query_filters.get("actor_id")
    if resolved_actor_id:
        effective_query_filters["actor_id"] = resolved_actor_id

    return base_metadata_template, effective_query_filters


# 把一次请求里的身份/会话相关 ID 规范化成一个稳定的字符串，方便后续做缓存键、检索范围、日志标识或 session 隔离
def _build_session_scope(filters):
    """Build determinstic session scope string from entity IDs."""
    parts = []
    # sorted 固定遍历顺序
    for key in sorted(["user_id", "agent_id", "run_id"]):
        val = filters.get(key)
        if val:
            parts.append(f"{key}={val}")
    return "&".join(parts)

setup_config()
logger = logging.getLogger(__name__)

class Memory(MemoryBase):
    def __init__(self, config: MemoryConfig = MemoryConfig()):
