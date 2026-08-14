from typing import Optional

from pydantic import BaseModel, Field, model_validator
from typing_extensions import Dict


class VectorStoreConfig(BaseModel):
    provider: str = Field(
        description="Provider of the vector store (e.g., 'qdrant', 'chroma', 'upstash_vector')",
        default="qdrant",
    )
    config: Optional[Dict] = Field(description="Configuration for the specific vector store", default=None)

    # provider 名称到配置类名的映射
    _provider_configs: Dict[str, str] = {
        "qdrant": "QdrantConfig",
        "chroma": "ChromaDbConfig",
        "pgvector": "PGVectorConfig",
        "pinecone": "PineconeConfig",
        "mongodb": "MongoDBConfig",
        "milvus": "MilvusDBConfig",
        "baidu": "BaiduDBConfig",
        "cassandra": "CassandraConfig",
        "neptune": "NeptuneAnalyticsConfig",
        "upstash_vector": "UpstashVectorConfig",
        "azure_ai_search": "AzureAISearchConfig",
        "azure_mysql": "AzureMySQLConfig",
        "redis": "RedisDBConfig",
        "valkey": "ValkeyConfig",
        "databricks": "DatabricksConfig",
        "elasticsearch": "ElasticsearchConfig",
        "vertex_ai_vector_search": "GoogleMatchingEngineConfig",
        "opensearch": "OpenSearchConfig",
        "supabase": "SupabaseConfig",
        "weaviate": "WeaviateConfig",
        "faiss": "FAISSConfig",
        "langchain": "LangchainConfig",
        "s3_vectors": "S3VectorsConfig",
        "turbopuffer": "TurbopufferConfig",
    }

    # 这个校验发生在 模型初始化完成之后，mode="after" 表示
    # 先把 provider 、 config 这些字段按正常规则解析完，再调用这个函数做进一步处理
    @model_validator(model="after")
    # self 说明这里拿到的是 已经构造好的模型实例
    def validate_and_create_config(self) -> "VectorStoreConfig":
        provider = self.provider
        config = self.config

        if provider not in self._provider_configs:
            raise ValueError(f"Unsupported vector store provider: {provider}")

        # 拼出模块路径，再从模块里取出对应类
        module = __import__(
            f"mem0.configs.vector_stores.{provider}",
            fromlist=[self._provider_configs[provider]]
        )
        config_class = getattr(module, self._provider_configs[provider])

        if config is None:
            config = {}

        # 如果 config 不是字典，检查它是不是已经是正确的配置对象
        if not isinstance(config, dict):
            if not isinstance(config, config_class):
                raise ValueError(f"Invalid config type for provider {provider}")
            return self

        # 如果配置类需要 path ，但用户没传，就自动补默认值
        if "path" not in config and "path" in config_class.__annotations__:
            config["path"] = f"/tmp/{provider}"

        # 把原始字典转换成 config 实例
        self.config = config_class(**config)
        return self
