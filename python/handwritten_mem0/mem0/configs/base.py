from optparse import Option
from turtle import update
from typing import Any

from pydantic import BaseModel, Field
from typing_extensions import Dict, Optional

# 这里的写法是 Pydantic 的用法，在类中定义属性，由 Pydantic 来完成初始化，同时进行校验
# Pydantic 主要用于做三件事：
# - 定义数据结构
# - 校验数据是否符合预期
# - 把输入数据转换成你想要的 Python 对象
class MemoryItem(BaseModel):
    id: str = Field(..., description="The unique identifier for the next data")
    memory: str = Field(
        ...,description="The memory deduced from the text data"
    )
    hash: Optional[str] = Field(None, description="The hash of the memory")
    metadata: Optional[Dict[str, Any]] = Field(None, description="Additonal metadata for the next data")
    score: Optional[float] = Field(None, description="The score associated with the text data")
    created_at: Optional[str] = Field(None, description="The timestamp when the memory was created")
    updated_at: Optional[str] = Field(None, description="The timestamp when the memory was updated")

class MemoryConfig(BaseModel):
    vector_store: VectorStoreConfig = Field(
        description="Configuration for the vector store",
        default_factory=VectorStoreConfig,
    )