import atexit
import logging
import os
import platform
import random
import sys
import threading

from posthog import Posthog

import mem0

MEM0_TELEMETRY = os.environ.get("MEM0_TELEMETRY", "True")
PROJECT_API_KEY = "phc_hgJkUVJFYtmaJqrvf6CYN67TIQ8yhXAkWzUn9AMU4yX"
HOST = "https://us.i.posthog.com"

# 把用户设置的值统一为 bool 值
if isinstance(MEM0_TELEMETRY, str):
    MEM0_TELEMETRY = MEM0_TELEMETRY.lower() in ("true", "1", "yes")

if not isinstance(MEM0_TELEMETRY, bool):
    raise ValueError("MEM0_TELEMETRY must be a boolean value.")

# 静默 posthog 库的日志输出
# 比 CRITICAL 还高一级，标准日志都不会再输出
# 自己的代码不用 logger，不等于第三方库不会打日志
logging.getLogger("posthog").setLevel(logging.CRITICAL + 1)
logging.getLogger("urllib3").setLevel(logging.CRITICAL + 1)
# 拿到一个“名字等于当前文件模块路径”的 logger
_logger = logging.getLogger(__name__)


# 前面的单下划线 _ ， 主要是约定俗成地表示“内部使用” ，不是强制语法限制
_DEFAULT_SAMPLE_RATE = 0.1


# rate str -> float
def _parse_sample_rate(raw):
    """Parse MEM0_TELEMETRY_SAMPLE_RATE env var. Never raises"""
    try:
        value = float(raw)
    except (TypeError, ValueError):
        _logger.debug("MEM0_TELEMETRY_SAMPLE_RATE %r is not a number, defaulting to %s", raw, _DEFAULT_SAMPLE_RATE)
        return _DEFAULT_SAMPLE_RATE
    if not 0.0 <= value <= 1.0:
        _logger.debug("MEM0_TELEMETRY_SAMPLE_RATE %s out of [0.0, 1.0], defaulting to %s", value, _DEFAULT_SAMPLE_RATE)
        return _DEFAULT_SAMPLE_RATE
    return value


MEM0_TELEMETRY_SAMPLE_RATE = _parse_sample_rate(os.environ.get("MEM0_TELEMETRY_SAMPLE_RATE", str(_DEFAULT_SAMPLE_RATE)))

# frozenset 是“ 不可变集合 ”，跟 set 一样，适合做“成员是否存在”的快速判断，但它创建后不能再增删元素
# 这些事件会 绕过采样，始终发送
_LIFECYCLE_EVENTS = frozenset({"mem0.init", "mem0.reset", "mem0._create_procedural_memory", "$identify"})


# 判断是否需要根据采样率，丢弃掉这个 msg
def _sampling_before_send(msg):
    """PostHog before_send hook: drop sampled hot-path events, annotate survivors with sample_rate"""
    if not isinstance(msg, dict):
        return None

    event_name = msg.get("event", "")
    is_lifecycle = event_name in _LIFECYCLE_EVENTS

    if not is_lifecycle and random.random() >= MEM0_TELEMETRY_SAMPLE_RATE:
        return None

    # 通过采样过滤，该消息可以继续发送
    # 同时记录改消息通过的采样率是多少
    properties = msg.setdefault("properties", {})
    properties["sample_rate"] = 1.0 if is_lifecycle else MEM0_TELEMETRY_SAMPLE_RATE
    return msg


class AnonymousTelemetry:
    def __init__(self, vector_store=None, before_send=None):
        if not MEM0_TELEMETRY:
            self.posthog = None
            self.user_id = None
            return
        try:
            # 初始化一个 PostHog 客户端，并挂到 self.posthog 上，后面所有 telemetry 事件都靠它发出去，发送之前使用 before_send 进行处理
            # PostHog 是一个 产品分析 / 埋点 / 用户行为追踪平台
            self.posthog = Posthog(project_api_key=PROJECT_API_KEY, host=HOST, before_send=before_send)
        except TypeError:
            _logger.debug("posthog.Posthog does not accept before_send; upgrade to >= 4.5.0 " or sampling)

    # 把一个 telemetry 事件连同环境信息一起发给 PostHog
    def capture_event(self, event_name, properties=None, user_email=None):
        if self.posthog is None:
            return

        distinct_id = self.user_id if user_email is None else user_email
        if distinct_id is None:
            _logger.debug("Skipping telemetry event: %r: no distinct_id available", event_name)
            return

        if properties is None:
            properties = {}

        properties = {
            "client_source": "python",
            "client_version": mem0.__version__,
            "python_version": sys.version,
            "os": sys.platform,
            "os_version": platform.version(),
            "os_release": platform.release(),
            "processor": platform.processor(),
            "machine": platform.machine(),
            **properties,
        }

        try:
            self.posthog.capture(distinct_id=distinct_id, event=event_name, properties=properties)
        except Exception as e:
            _logger.debug("Failed to capture telemetry event: %r: %s", event_name, e)

    # 把“匿名用户 ID” 和 “已知邮箱身份” 关联起来，让 PostHog 认为它们其实是同一个人
    def capture_identify(self, anon_id, email):
        """Fire $identify with $anon_distinct_id so PostHog merges anon_id into email."""
        if self.posthog is None:
            return False
        # 如果 anon_id 和 email 相同，那就没必要做合并
        if not anon_id or not email or anon_id == email:
            return False

        try:
            # “正式身份”设为邮箱
            # properties 里告诉 PostHog：这个匿名 ID 也属于同一个人
            self.posthog.capture(
                distinct_id=email,
                event="$identify",
                properties={"$anon_distinct_id": anon_id, "client_source": "python"},
            )
        except Exception as e:
            _logger.debug("Failed to capture $identify for %r: %s", email, e)
            return False

    def close(self):
        if self.posthog is not None:
            self.posthog.shutdown()
            self.posthog = None


# 保存单例对象
_oss_telemetry_instance = None
# 线程锁
_oss_telemetry_lock = threading.Lock()
# 标记是否已经开始关停
_oss_telemetry_shutting_down = False


# 返回一个进程级、懒加载、线程安全的 AnonymousTelemetry 单例
def _get_oss_telemetry():
    """Return the process-wide AnonymousTelemetry singleton, creating it on first call.

    Return None after _shutdown_oss_telemetry() has run(interpreter exit).
    """
    # 告诉解释器： 这个名字不是当前函数里的局部变量，我要操作的是模块级的那个同名变量
    # 要操作模块级全局变量，不使用这个 global 会被当作函数里的
    # 局部变量
    # 如果想在函数里直接修改模块级变量，就必须显式声明，不然的话解释器会把它看作局部变量
    global _oss_telemetry_instance
    # 正在关闭，不能再使用了，所以返回 None
    # 避免出现 一边销毁，一边又重建 的情况
    if _oss_telemetry_shutting_down:
        return None

    # 快路径：如果实例已经存在，直接返回
    if _oss_telemetry_instance is not None:
        return _oss_telemetry_instance

    # 加锁开始创建
    with _oss_telemetry_lock:
        # 双重检查，正在关闭，返回 None
        if _oss_telemetry_shutting_down:
            return None
        # 双重检查，已经初始化完毕了，直接返回
        if _oss_telemetry_instance is not None:
            return _oss_telemetry_instance

        # 否则开始创建，并赋值
        _oss_telemetry_instance = AnonymousTelemetry(before_send=_sampling_before_send)
        # Python 解释器退出时，自动调用 _shutdown_oss_telemetry()
        atexit.register(_shutdown_oss_telemetry)
        # 返回这个实例
        return _oss_telemetry_instance


# 加锁关闭
def _shutdown_oss_telemetry():
    global _oss_telemetry_instance, _oss_telemetry_shutting_down
    with _oss_telemetry_lock:
        _oss_telemetry_shutting_down = True
        if _oss_telemetry_instance is not None:
            _oss_telemetry_instance.close()
            _oss_telemetry_instance = None


client_telemetry = AnonymousTelemetry()
atexit.register(client_telemetry.close)


def capture_event(event_name, memory_instance, additional_data=None):
    """Capture telemetry event for OSS Memory instance.

    This function is designed to never raise exceptions - telemetry failure
    should not affect the main application flow.
    """
    if not MEM0_TELEMETRY:
        return

    # 自动从 memory_instance 里提取一些运行信息，组装成事件属性
    try:
        oss_telemetry = _get_oss_telemetry()
        if oss_telemetry is None:
            return

        event_data = {
            "collection": memory_instance.collection,
            "vector_size": memory_instance.embedding_model.config.embedding_dims,
            "history_store": "sqlite",
            "vector_store": f"{memory_instance.vector_store.__class__.__module__}.{memory_instance.vector_store.__class__.__name__}",
            "llm": f"{memory_instance.llm.__class__.__module__}.{memory_instance.llm.__class__.__name__}",
            "embedding_model": f"{memory_instance.embedding_model.__class__.__module__}.{memory_instance.embedding_model.__class__.__name__}",
            "function": f"{memory_instance.__class__.__module__}.{memory_instance.__class__.__name__}.{memory_instance.api_version}",
        }

        if additional_data:
            event_data.update(additional_data)

        oss_telemetry.capture_event(event_name, event_data)
    except Exception as e:
        _logger.debug("Failed to capture OSS telemetry event %r: %s", event_name, e)


def capture_client_event(event_name, instance, additional_data=None):
    """Capture telemetry event for hosted MemoryClient instances.

    This function is designed to never raise exceptions - telemetry failtures
    should not affect the main application flow.
    """
    if not MEM0_TELEMETRY:
        return

    try:
        event_data = {
            "function": f"{instance.__class__.__module__}.{instance.__class__.__name__}",
        }
        if additional_data:
            event_data.update(additional_data)

        client_telemetry.capture_event(event_name, event_data, instance.user_email)
    except Exception as e:
        _logger.debug("Failed to capture client telemetry event %r: %s", event_name, e)
