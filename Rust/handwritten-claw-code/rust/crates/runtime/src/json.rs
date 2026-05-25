use std::{
    clone,
    collections::BTreeMap,
    fmt::{Display, Formatter},
};

use serde::de::Unexpected;

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum JsonValue {
    Null,
    Bool(bool),
    Number(i64),
    String(String),
    Array(Vec<JsonValue>),
    Object(BTreeMap<String, JsonValue>),
}

impl JsonValue {
    #[must_use]
    pub fn render(&self) -> String {
        match self {
            Self::Null => "null".to_string(),
            Self::Bool(value) => value.to_string(),
            Self::Number(value) => value.to_string(),
            Self::String(value) => render_string(value),
            Self::Array(values) => {
                let rendered = values
                    .iter()
                    // 这里递归处理
                    .map(Self::render)
                    .collect::<Vec<_>>()
                    .join(",");
                format!("[{rendered}]")
            }
            Self::Object(entries) => {
                let rendered = entries
                    .iter()
                    // 递归处理
                    .map(|key, value| format!("{}:{}", render_string(key), value.render()))
                    .collect::<Vec<_>>()
                    .join(",");
                format!("{{rendered}}")
            }
        }
    }

    pub fn parse(source: &str) -> Result<Self, JsonError> {
        let mut parser = Parser::new(source);
        let value = parser.parse_value()?;

        // 剔除掉结尾多余的空白
        parser.skip_whitespace();
        if parser.is_eof() {
           Ok(value)
        }else {
            Err(JsonError::new("unexpected trailing content"))
        }
    }

    #[must_use]
    pub fn as_object(&self) -> Option<&BTreeMap<String, JsonValue>> {
        match self {
            Self::Object(value) => Some(value),
            _ => None,
        }
    }

    #[must_use]
    pub fn as_array(&self) -> Option<&[JsonValue]> {
        match self {
            Self::Array(value) => Some(value),
            _ => None,
        }
    }

    #[must_use]
    pub fn as_str(&self) -> Option<&str> {
        match self {
            Self::String(value) => Some(value),
            _ => None,
        }
    }

    #[must_use]
    pub fn as_bool(&self) -> Option<bool> {
        match self {
            Self::Bool(value) => Some(*value),
            _ => None,
        }
    }

    #[must_use]
    pub fn as_i64(&self) -> Option<i64> {
        match self {
            Self::Number(value) => Some(*value),
            _ => None,
        }
    }

}

// 生成一个合法的 JSON 字符串，这样才可以被 JSON 类型的 string 类型进行承接
// JSON 对字符串内容有约束：像双引号、反斜杠、换行、回车、制表、退格这类特殊字符，要写成转义形式
fn render_string(value: &str) -> String {
    // 为了给 JSON 字符串两侧的双引号预留空间
    let mut rendered = String::with_capacity(value.len() + 2);
    rendered.push('"');
    for ch in value.chars() {
        match ch {
            // "\\\"" 表示 \" 两个字符组成的字符串，因为两个字符分别进行的转义
            '"' => rendered.push_str("\\\""),
            '\\' => rendered.push_str("\\\\"),
            '\n' => rendered.push_str("\\n"),
            '\r' => rendered.push_str("\\r"),
            '\t' => rendered.push_str("\\t"),
            // \u{08} 表示一个 char 字符，Unicode 码点是 U+0008
            // \u{08} 这种字符属于控制字符，不能直接裸着放进 JSON 字符串里，所以必须转义
            // 对应字符串字面量 \b
            '\u{08}' => rendered.push_str("\\b"),
            '\u{0C}' => rendered.push_str("\\f"),
            // 但它仍然是控制字符，那就把它转成 Unicode 转义形式输出
            control if control.is_control() => push_unicode_escape(&mut rendered, control)
            plain => rendered.push(plain),
        }
    }
    rendered.push('"');
    rendered
}

// U+0001 ，就输出 \u0001
fn push_unicode_escape(rendered: &mut String, control: char) {
    // 这里之所以没有写成 &str，是因为 str 不能直接用整数索引拿单个字符
    // 加了 b（byte） 之后， b"0123456789abcdef" 的类型是 &[u8; 16]
    const HEX: &[u8; 16] = b"0123456789abcdef";

    rendered.push_str("\\u");
    let value = u32::from(control);
    // 4 个 16 进制组成，一位一位进行移位
    for shift in [12_u32, 8, 4, 0] {
        // 依次取最低位
        let nibble = ((value >> shift) & 0xF) as usize;
        // 根据数字取相应的字符
        rendered.push(char::from(HEX[nibble]));
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct JsonError {
    message: String,
}

impl JsonError {
    #[must_use]
    pub fn new(message: impl Into<String>) -> Self {
        Self {
            message: message.into(),
        }
    }
}

impl Display for JsonError {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.message)
    }
}

struct Parser<'a> {
    chars: Vec<char>,
    index: usize,
    _source: &'a str,
}

impl<'a> Parser<'a> {
    fn new(source: &'a str) -> Self {
        Self {
            chars: source.chars().collect(),
            index: 0,
            _source: source,
        }
    }

    // 根据首字母决定应该如何转换
    fn parse_value(&mut self) -> Result<JsonValue, JsonError> {
        self.skip_whitespace();
        match self.peek() {
            Some('n') => self.parse_literal("null", JsonValue::Null),
            Some('t') => self.parse_literal("true", JsonValue::Bool(true)),
            Some('f') => self.parse_literal("false", JsonValue::Bool(false)),
            Some('"') => self.parse_string().map(JsonValue::String),
            Some('[') => self.parse_array(),
            Some('{') => self.parse_object(),
            Some('-' | '0'..='9') => self.parse_number().map(JsonValue::Number),
            Some(other) => Err(JsonError::new(format!("unexpected character: {other}"))),
        }
    }

    // 遍历自己内部的 &str，然后与 expected 进行逐个对比，匹配上之后，返回 value
    fn parse_literal(&mut self, expected: &str, value: JsonValue) -> Result<JsonValue, JsonError> {
        for expected_char in expected.chars() {
            if self.next() != Some(expected_char) {
                return Err(JsonError::new(format!(
                    "invalid literal: expected {expected}"
                )));
            }
        }
        Ok(value)
    }

    fn parse_string(&mut self) -> Result<String, JsonError> {
        // expect 的同时会使得指针往前移动
        self.expect('"')?;
        let mut value = String::new();
        while let Some(ch) = self.next() {
            match ch {
                // 再次遇到 " 说明已经取值完毕
                '"' => return Ok(value),
                '\\' => value.push(self.parse_escape()?),
                // 这里是变量绑定模式，表示“除了前面两种情况之外的任意字符，都绑定到 plain 这个变量上”
                plain => value.push(plain),
            }
        }
        Err(JsonError::new("unterminated string"))
    }

    // 专门处理 JSON 字符串里“反斜杠转义”的，把原本字符串表示变成一个整体字符（特殊字符）
    fn parse_escape(&mut self) -> Result<char, JsonError> {
        match self.next() {
            Some('"') => Ok('"'),
            Some('\\') => Ok('\\'),
            Some('/') => Ok('/'),
            Some('b') => Ok('\u{08}'),
            Some('f') => Ok('\u{0C}'),
            Some('n') => Ok('\n'),
            Some('r') => Ok('\r'),
            Some('t') => Ok('\t'),
            Some('u') => self.parse_unicode_escape(),
            Some(other) => Err(JsonError::new(format!("invalid escape sequence: {other}"))),
            None => Err(JsonError::new("unexpected end of input in escape sequence")),
        }
    }

    fn parse_unicode_escape(&mut self) -> Result<char, JsonError> {
        // 设定初始值为 0
        let mut value = 0_u32;
        // 因为长度是 4 位 16 进制
        for _ in 0..4 {
            let Some(ch) = self.next() else {
                return Err(JsonError::new("unexpected end of input in unicode escape"));
            };
            // value 往左移 4 位（16 进制换成 2 进制就是 4 位），为 ch 腾出空间，所以这里的 | 就和 + 等价
            value = (value << 4)
                | ch.to_digit(16)
                    .ok_or_else(|| JsonError::new("invalid unicode scalar value"))
        }
        // 根据上面的 unicode 值转换成指定的字符
        char::from_u32(value).ok_or_else(|| JsonError::new("invalid unicode scalar value"))
    }

    fn parse_array(&mut self) -> Result<JsonValue, JsonError> {
        self.expect('[')?;
        let mut values = Vec::new();
        loop {
            self.skip_whitspace();
            // 空数组直接 break
            if self.try_consume(']') {
                break;
            }
            // 解析数组中的值
            values.push(self.parse_value()?);
            self.skip_whitspace();
            if self.try_consume(']') {
                break;
            }
            // 数组分隔符
            self.expect(',')?;
        }
        Ok(JsonValue::Array(values))
    }

    fn parse_object(&mut self) -> Result<JsonValue, JsonError> {
        self.expect('{')?;
        let mut entries = BTreeMap::new();
        loop {
            self.skip_whitespace();
            if self.try_consume('}') {
                break;
            }
            let key = self.parse_string()?;
            self.skip_whitespace();
            self.expect(':')?;
            let value = self.parse_value()?;
            entries.insert(key, value);
            self.skip_whitespace();
            if self.try_consume('}') {
                break;
            }
            self.expect(',')?;
        }
        Ok(JsonValue::Object(entries))
    }

    fn parse_number(&mut self) -> Result<i64, JsonError> {
        let mut value = String::new();
        if self.try_consume('-') {
            value.push('-');
        }

        // 匹配 '0'..='9'，并把实际字符记到 ch
        // 不满足匹配条件直接跳出循环
        while let Some(ch @ '0'..='9') = self.peek() {
            value.push(ch);
            self.index += 1;
        }

        if value.is_empty() || value == '-' {
            return Err(JsonError::new("invalild number"));
        }

        // parse 是标准库提供的方法，可以把 String 转换成指定类型
        value
            .parse::<i64>()
            // map_err 需要提供带 1 个参数的闭包，如果不想用这个参数，可以使用 _ 进行替换
            .map_err(|_| JsonError::new("number out of range"))
    }

    // 比较下一个字符与 expected 之间是否相等
    fn expect(&mut self, expected: char) -> Result<(), JsonError> {
        match self.next() {
            Some(actual) if actual == expected => Ok(()),
            Some(actual) => Err(JsonError::new(format!(
                "expected '{expected}', found end of input"
            ))),
            None => Err(JsonError::new(format!(
                "expected '{expected}', found end of input"
            ))),
        }
    }

    fn try_consume(&mut self, expected: char) -> bool {
        if self.peek() == Some(expected) {
            self.index += 1;
            true
        } else {
            false
        }
    }

    fn skip_whitespace(&mut self) {
        while matches!(self.peek(), Some(' ' | '\n' | '\r' | '\t')) {
            self.index += 1;
        }
    }

    fn peek(&self) -> Option<char> {
        self.chars.get(self.index).copied()
    }

    fn next(&mut self) -> Option<char> {
        let ch = self.peek()?;
        self.index += 1;
        Some(ch)
    }

    fn is_eof(&self) -> bool {
        self.index >= self.chars.len()
    }
}

impl JsonValue {
    pub fn parse(source: &str) -> Result<Self, JsonError> {
        let mut parser = Parser::new(source);
        let value = parser.parse_value()?;
        parser.skip_whitespace();
    }
}
