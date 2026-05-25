use serde::{Deserialize, Serialize};
use serde_json::Value as JsonValue;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct McpTool {
    pub name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    #[serde(rename = "inputSchema", skip_serializing_if = "Option::is_none")]
    pub input_schema: Option<JsonValue>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub annotations: Option<JsonValue>,
    #[serde(rename = "_meta", skip_serializing_if = "Option::is_none")]
    pub meta: Option<JsonValue>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct JsonRpcRequest<T = JsonValue>  {
    pub jsonrpc: String,
    pub id: JsonRpcId,
    pub method: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub params: Option<T>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
// T = JsonValue 泛型默认类型参数 语法，当 T 未指定时，默认使用 JsonValue 类型
pub struct JsonRpcResponse<T = JsonValue> {
    pub jsonrpc: String,
    pub id: JsonRpcId,
    pub result: Option<T>,
    pub error: Option<JsonRpcError>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
// 告诉 serde 这个 enum 在序列化/反序列化时，不要额外加“类型标签
// 解析时根据数据本身的形状，依次尝试匹配各个枚举分支
// 如果没有 untagged ，就会很别扭，serde 往往会期待这种“带标签”的 JSON：
//  {"Number": 123 } 或者 { "String": "abc-123" }
#[serde(untagged)]
pub enum JsonRpcId {
    Number(u64),
    String(String),
    Null,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct  JsonRpcError {
    pub code: i64,
    pub message: String,
    // 如果没有，就干脆别输出这个字段
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<JsonValue>,
}