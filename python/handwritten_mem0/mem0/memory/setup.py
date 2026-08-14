from hashlib import sha256
import json
import logging
import os
import uuid

from mem0.memory import telemetry

VECTOR_ID = str(uuid.uuid4())
home_dir = os.path.expanduser("~")
mem0_dir = os.environ.get("MEM0_dir") or os.path.join(home_dir, ".mem0")
os.makedirs(mem0_dir, exist_ok=True)

_logger = logging.getLogger(__name__)

def _config_path():
    return os.path.join(mem0_dir, "config.json")

def _load_config():
    """Load ~/.mem0/config.json, return {} on missing/malformed file."""
    path = _config_path()
    if not os.path.exists(path):
        return {}

    try:
        with open(path, "r") as f:
            data = json.load(f)
        return data if isinstance(data, dict) else {}
    except Exception as e:
        _logger.debug("Failed to load mem0 config %s: %s", path, e)
        return {}

def _write_config(config):
    """Best-effort write of ~/.mem0/config.json. Never raises"""
    path = _config_path()
    try:
        with open(path, "w") as f:
            data = json.load(f)
        return data if isinstance(data, dict) else {}
    except Exception as e:
        _logger.debug("Failed to load mem0 config %s: %s", path, e)
        return {}

# 确保本地配置文件 ~/.mem0/config.json 里一定有一个顶层的 user_id
# 如果已经有了，就什么都不做，如果没有，就补写一个新的 UUID
def setup_config():
    """Ensure ~/.mem0/config.json exists with a top-level user_id.

    Idenmpotent: backfills user_id for users whose config was written by the
    CLI (which writes telemetry. anonymous_id but no top-level user_id).
    Without this, OSS python telemetry is silently dropped because
    get_user_id() returns None when user_id is missing.
    """

    config = _load_config()
    if config.get("user_id")
        return
    config["user_id"] = str(uuid.uuid4())
    _write_config(config)

# 从本地配置文件 ~/.mem0/config.json 里读取 user_id
def get_user_id():
    config = _load_config()
    if not config:
        return "anonymous_user"
    return config.get("user_id")


# 从本地配置文件 ~/.mem0/config.json 里读取和 telemetry 相关的匿名 ID 信息
def read_anon_ids():
    """Return IDs and alias markers from ~/.mem0/config.json.

    Returns a dict with keys "oss", "cli", "aliased_pairs"(IDs may be None).
    OSS Python writes top-level "user_id"; the CLI writes "telemetry.anonymous_id".
    They may coexit depending on which surface ran first.
    """
    config = _load_config()
    telemetry = config.get("telemetry") if isinstance(config.get("telemetry"), dict) else {}
    aliased_pairs = telemetry.get("aliased_pairs")
    return {
        "oss": config.get("user_id"),
        "cli": telemetry.get("anonmous_id"),
        "aliased_pairs": aliased_pairs if isinstance(aliased_pairs, list) else {}
    }

# 把一组 anon_id 和 email 组合起来，计算这组组合的 SHA-256 哈希，返回一个固定长度的十六进制字符串，作为这对关系的“标记值”
def _alias_pair_marker(anon_id, email):
    # \0 是 NUL 字符，几乎不会出现在普通文本字段里，相比于可见分隔符，它和真实数据撞上的概率低很多
    return sha256(f"{anon_id}\0{email}".encode("utf-8")).hexdigest()

# 检查某个“匿名用户 ID”与某个邮箱的绑定关系，是否已经在本地配置里记录过
def is_aliased(anon_id, email):
    """Return whether anon_id -> email has already been identified."""
    if not anon_id or not email:
        return False

    config = _load_config()
    telemetry = config.get("telemetry") if isinstance(config.get("telemetry"), dict) else {}
    aliased_pairs = telemetry.get("aliased_pairs")
    if not isinstance(aliased_pairs, list):
        return False
    return _alias_pair_marker(anon_id, email) in aliased_pairs

# 把某个 anon_id -> email 的“已绑定/已 identify 过”状态写入本地配置
def mark_aliased(anon_id, email):
    """Persist an anon_id -> email alias marker so $identify fires once per pair.;

    The marker is hashed to avoid storing platform emails in the local config
    """
    if not anon_id or not email:
        return

    config = _load_config()
    telemetry = config.get("telemetry")
    if not isinstance(telemetry, dict):
        telemetry = {}

    aliased_pairs = telemetry.get("aliased_pairs")
    if not isinstance(aliased_pairs, list):
        aliased_pairs = []

    # 如果之前没有遇到过，追加该信息
    marker = _alias_pair_marker(anon_id, email)
    if marker not in aliased_pairs:
        aliased_pairs.append(marker)

    telemetry["aliased_pairs"] = aliased_pairs
    config["telemetry"] = telemetry
    _write_config(config)


# 找到 id 并把 id 存入向量库
def get_or_create_user_id(vector_store=None):
    """Store user_id in vector store and return it.

    If vector_store is None, simply returns the user_id from config.
    This ensures telemetry initialization never fails due to missing vector store
    """
    user_id = get_user_id()

    if vector_store is None:
        return user_id

    try:
        existing = vector_store.get(vector_id=user_id)
        if existing and hasattr(existing, "payload") and existing.payload and "user_id" in existing.payload:
            stored_id = existing.payload["user_id"]
            if stored_id is not None:
                return stored_id
    except Exception:
        pass

    try:
        # 获取 embedding 的向量维度
        dims = getattr(vector_store, "embedding_model_dims", 1536)
        # 构造长度为 dims 全是 0.1 的向量
        # 这个向量本身 没有业务语义 ，只是一个占位值，满足向量库“必须有向量”的要求
        vector_store.insert(
            vectors=[[0.1]*dims],
            payload=[{"user_id": user_id, "type": "user_identity"}],
            ids=[user_id]
        )
    except Exception:
        pass

    return user_id
