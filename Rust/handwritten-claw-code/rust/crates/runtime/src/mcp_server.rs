use std::collections::BTreeMap;
use std::io;
use serde::{Deserialize, Serialize};
use crate::{JsonRpcError, JsonRpcId, JsonRpcResponse, McpTool};
use serde_json::{json, Value as JsonValue};
// stdin, stdout 是函数，Stdin, Stdout 是类型
use tokio::io::{BufReader, stdin, stdout, Stdin, Stdout, AsyncWriteExt, AsyncBufReadExt, AsyncReadExt};
use crate::mcp_studio::JsonRpcRequest;

pub const MCP_SERVER_PROTOCOL_VERSION: &str = "2025-03-26";

// 给工具处理函数定义一个统一的别名
// dyn Fn(...) 表示“某个实现了这个函数接口的对象”，也就是动态分发的闭包/函数对象
// 因为 dyn Fn 是 trait object，编译期大小不固定，要放到堆上，所以需要 Box 包起来
// + Send + Sync 表示这个函数可以安全地在多个线程之间传递
// + 'static 要求这个闭包不要借用短生命周期的数据，或者说它持有的数据要能活得足够久
pub type ToolCallHandler =
Box<dyn Fn(&str, &JsonValue) -> Result<String, String> + Send + Sync + 'static>;

pub struct McpServerSpec {
    pub server_name : String,
    pub server_version: String,
    pub tools: Vec<McpTool>,
    pub tool_handler: ToolCallHandler,
}

pub struct McpServer {
    spec: McpServerSpec,
    stdin: BufReader<Stdin>,
    stdout: Stdout,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "camelCase")]
pub struct McpInitializeResult {
    pub protocol_version: String,
    pub capabilities: JsonValue,
    pub server_info: McpInitializeServerInfo,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "camelCase")]
pub struct McpInitializeServerInfo {
    pub name: String,
    pub version: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "camelCase")]
pub struct McpListToolsResult {
    pub tools: Vec<McpTool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub next_cursor: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "camelCase")]
pub struct McpToolCallParams {
    pub name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub arguments: Option<JsonValue>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub meta: Option<JsonValue>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "camelCase")]
pub struct McpToolCallResult {
    #[serde(default)]
    pub content: Vec<McpToolCallContent>,
    #[serde(default)]
    pub structured_content: Option<JsonValue>,
    #[serde(default)]
    pub is_error: Option<bool>,
    #[serde(rename = "_meta", skip_serializing_if = "Option::is_none")]
    pub meta: Option<JsonValue>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct McpToolCallContent {
    #[serde(rename = "type")]
    pub kind: String,
    // 加了 #[serde(flatten)] 之后， data 里的键值会直接并入外层对象
    #[serde(flatten)]
    pub data: BTreeMap<String, JsonValue>,
}

impl McpServer {
    #[must_use]
    pub fn new(spec: McpServerSpec) -> Self {
        Self {
            spec,
            stdin: BufReader::new(stdin()),
            stdout: stdout(),
        }
    }

    pub async fn run(&mut self) -> io::Result<()> {
        loop {
            let Some(payload) = read_frame(&mut self.stdin).await? else {
               return Ok(());
            };

            let message : JsonValue = match serde_json::from_slice(&payload) {
                Ok(value) => value,
                Err(error) => {
                    let response = JsonRpcResponse::<JsonValue> {
                       jsonrpc: "2.0" .to_string(),
                        id: JsonRpcId::Null,
                        result: None,
                        error: Some(JsonRpcError{
                            code: -32700,
                            message:format!("parse error: {}", error),
                            data: None,
                        }),
                    };
                    write_response(&mut self.stdout, &response).await?;
                    continue;
                }
            };

            // 如果收到的 message 里没有 id 字段，就把它当成通知型消息处理，不返回任何响应
            if message.get("id").is_none() {
                continue;
            }

            let request: JsonRpcRequest<JsonValue> = match serde_json::from_value(message) {
                Ok(request) => request,
                Err(error) => {
                    let response = JsonRpcResponse::<JsonValue> {
                        jsonrpc: "2.0".to_string(),
                        id: JsonRpcId::Null,
                        result: None,
                        error: Some(JsonRpcError{
                            code: -32600,
                            message: format!("invalid request: {}", error),
                            data: None,
                        }),
                    };
                    write_response(&mut self.stdout, &response).await?;
                    continue;
                }
            };

            let response = self.dipatch(request);
            write_response(&mut self.stdout, &response).await?;

        }

    }

    fn dispatch(&self, request:JsonRpcRequest<JsonValue> ) -> JsonRpcResponse<JsonValue> {
        let id = request.id.clone();
        match request.method.as_str() {
           "initialize" => self.handle_initialize(id),
            "tools/list" => self.handle_tools_list(id),
            "tools/call" => self.handle_tools_call(id, request.params),
            other => JsonRpcResponse {
                jsonrpc: "2.0".to_string(),
                id,
                result: None,
                error: Some(JsonRpcError {
                    code: -32601,
                    message: format!("method not found: {other}"),
                    data: None,
                }),
            }
        }
    }

    fn handle_initialize(&self, id: JsonRpcId) -> JsonRpcResponse<JsonValue> {
        let result = McpInitializeResult {
            protocol_version: MCP_SERVER_PROTOCOL_VERSION.to_string(),
            capabilities: json!({"tools": {}}),
            server_info: McpInitializeServerInfo {
                name: self.spec.server_name.clone(),
                version: self.spec.server_version.clone(),
            }
        };
        JsonRpcResponse {
            jsonrpc: "2.0".to_string(),
            id,
            result: serde_json::to_value(result).ok(),
            error: None,
        }
    }

    fn handle_tools_list(&self, id: JsonRpcId) -> JsonRpcResponse<JsonValue> {
        let result = McpListToolsResult {
            tools: self.spec.tools.clone(),
            next_cursor: None,
        };
        JsonRpcResponse {
            jsonrpc: "2.0".to_string(),
            id,
            result: serde_json::to_value(result).ok(),
            error: None,
        }
    }

    fn handle_tools_call(&self, id: JsonRpcId, params: Option<JsonValue>) -> JsonRpcResponse<JsonValue> {
        let Some(params) = params else {
            return invalid_params_response(id, "missing params for tools/call");
        };
        let call: McpToolCallParams = match serde_json::from_value(params) {
            Ok(value) => value,
            Err(error) => {
                return invalid_params_response(id, &format!("invalid params: {}", error));
            }
        };
        // unwrap_or_else 用于兜底
        let arguments = call.arguments.unwrap_or_else(|| json!({}));
        let tool_result = (self.spec.tool_handler)(&call.name, &arguments);
        let (text, is_error) = match tool_result {
            Ok(result) => (result, false),
            Err(message) => (message, true),
        };
        // BTreeMap 的“有序”，指的是 按 key 的大小关系维护顺序 ，不是按插入顺序
        let mut data = std::collections::BTreeMap::new();
        data.insert("text".to_string(), JsonValue::String(text));
        let call_result = McpToolCallResult {
            content: vec![McpToolCallContent {
                kind: "text".to_string(),
                data,
            }],
            structured_content: None,
            is_error: Some(is_error),
            meta: None,
        };
        JsonRpcResponse {
            jsonrpc: "2.0".to_string(),
            id,
            result: serde_json::to_value(call_result).ok(),
            error: None,
        }

    }

}

fn invalid_params_response(id: JsonRpcId, message: &str) -> JsonRpcResponse<JsonValue> {
    JsonRpcResponse {
        jsonrpc: "2.0".to_string(),
        id,
        result: None,
        error: Some(JsonRpcError {
            code: -32602,
            message: message.to_string(),
            data: None,
        }),
    }
}

async fn read_frame(reader: &mut BufReader<Stdin>) -> io::Result<Option<Vec<u8>>> {
    let mut content_length : Option<usize> = None;
    let mut first_header = true;

    loop {
        let mut line = String::new();
        let bytes_read = reader.read_line(&mut line).await?;
        if bytes_read == 0 {
            // 连第一行 header 都还没开始读，说明对端是“正常结束”，当前也没有半包数据要处理
            if first_header {
                return Ok(None)
            }
            return Err(io::Error::new(
                io::ErrorKind::UnexpectedEof,
                "MCP stdio stream closed while reading headers",
            ))
        }
        // 已经有数据了，说明至少读取到一行 header 了，first_header 为 false
        first_header = false;
        // header 区域结束了 。
        // 在 HTTP/LSP/MCP 这种文本协议里，header 和 body 之间会有一个空行
        if line == "\r\n" || line == "\n" {
            break;
        }
        // 把字符串 line 末尾的 \r 和 \n 去掉
        let header = line.trim_end_matches(['\r', '\n']);

        if let Some((name, value)) = header.split_once(':') {
            if name.trim().eq_ignore_ascii_case("Content-Length") {
                let parsed = value
                    .trim()
                    .parse::<usize>()
                .map_err(|e| io::Error::new(io::ErrorKind::InvalidData, e))?;
                content_length = Some(parsed);
            }
        }
    }
    let content_length = content_length.ok_or_else(|| {
        io::Error::new(io::ErrorKind::InvalidData, "Content-Length missing")
    })?;
    let mut payload = vec![0_u8; content_length];
    reader.read_exact(&mut payload).await?;
    Ok(Some(payload))
}

async fn write_response(
    stdout: &mut Stdout,
    response: &JsonRpcResponse<JsonValue>,
) -> io::Result<()> {
    // 把 response 转成 Vec<u8>，如果失败，把错误包装成 io::Error 类型以契合函数返回类型
    let body = serde_json::to_vec(response)
        .map_err(|error| io::Error::new(io::ErrorKind::InvalidData, error))?;
    let header = format!("Content-Length: {}\r\n\r\n", body.len());
    stdout.write_all(header.as_bytes()).await?;
    stdout.write_all(&body).await?;
    stdout.flush().await
}