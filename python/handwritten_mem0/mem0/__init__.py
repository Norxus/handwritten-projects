"""Handwritten mem0 package."""
import importlib.metadata

# 去 Python 已安装包的元数据里查找名字叫 mem0ai 的发行包版本
__version__ = importlib.metadata.version("mem0ai")