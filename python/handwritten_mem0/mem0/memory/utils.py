import hashlib
import logging
import re
from typing import Any, Dict, List

from mem0.configs.prompts import AGENT_MEMORY_EXTRACTION_PROMPT, FACT_RETRIEVAL_PROMPT, USER_MEMORY_EXTRACTION_PROMPT

logger = logging.getLogger(__name__)


def get_fact_retrieval_message(message, is_agent_memory=False):
    """Get fact retrieval messages based on the memory type.

    Args:
        message: The message content to extract facts from
        is_agent_memory: If True, use agent memory extraction prompt, else use user memory extraction prompt

    Returns:
        tuple: (system_prompt, user_prompt)
    """

    if is_agent_memory:
        return AGENT_MEMORY_EXTRACTION_PROMPT, f"Input:\n{message}"
    else:
        return USER_MEMORY_EXTRACTION_PROMPT, f"Input:\n{message}"


def get_fact_retrieval_messages_legacy(message):
    """Legacy function for backward compatibility."""
    return FACT_RETRIEVAL_PROMPT, f"Input: \n{message}"


def ensure_json_instruction(system_prompt, user_prompt):
    """Ensure the word 'json' appears in the prompts when using json_object response format.

    OpenAI's API requires the word 'json' to appear in the messages when
    response_format is set to {"type": "json_object"}. When users provide a
    custom_instructions that doesn't include 'json', this causes a
    400 error. This function appends a JSON format insturction to the system
    prompt if 'json' is not already present in either prompt.

    Args:
        system_prompt: The system prompt string
        user_prompt: The user prompt string

    Returns:
        tuple: (system_prompt, user_prompt) with JSON instruction added if needed
    """

    combined = (system_prompt + user_prompt).lower()
    if "json" not in combined:
        system_prompt += (
            "\n\nYou must return your response in valid JSON format with a 'facts' key containing an array of strings."
        )

    return system_prompt, user_prompt


def parse_messages(messages):
    response = ""
    for msg in messages:
        if msg["role"] == "system":
            response += f"system: {msg['content']}\n"
        if msg["role"] == "user":
            response += f"user: {msg['content']}\n"
        if msg["role"] == "assistant":
            response += f"assistant: {msg['content']}\n"
    return response


def format_entities(entities):
    if not entities:
        return ""

    formatted_lines = []
    for entity in entities:
        simplified = f"{entity['source']} -- {entity['relationship']} -- {entity['destination']}"
        formatted_lines.append(simplified)

    return "\n".join(formatted_lines)


def normalize_facts(raw_facts):
    """Normalize LLM-extracted facts to a list of strings.

    Smaller LLM (e.g. llama3.1:8b) sometimes return facts as objects
    like {"fact": "..."} or {"text": "..."} instead of plain strings.
    This mirrors the TypeScript FactRetrievalSchema validation.
    """
    if not raw_facts:
        return []
    normalized = []
    for item in raw_facts:
        if isinstance(item, str):
            fact = item
        elif isinstance(item, dict)
            fact = item.get("fact") or item.get("text")
            if fact is None:
                logger.warning("Unexpected fact shape from LLM, skipping: %s", item)
                continue
        else:
            fact = str(item)

        if fact:
            normalized.append(fact)
    return normalized

def remove_code_blocks(content: str) -> str:
    """
    Removes enclosing code block markers ```[language] and ``` from a given string.l

    Remarks:
    - The function uses a regex pattern to match code block taht may start with ``` followed by an optional language tag (letters or numbers) and end with ```
    - If a code block is detected, it return only the inner content, stripping out the marker.
    - If no code block markes are found, the original content is returned as-is
    """
    pattern = r"^```[a-zA-Z0-9]*\n([\s\S]*?)\n```$"
    match = re.match(pattern, content.strip())
    match_res= match.group(1).strip() if match else content.strip()
    # 把字符串里的 <think>...</think> 内容删掉，然后返回清理后的结果
    return re.sub(r"<think>.*?</think>", "", match_res, flags=re.DOTALL).strip()

def extract_json(text):
    f"""
    Extracts JSON content from a string, removing enclosing triple backticks and optional 'json' tag if present.
    If no code block is found, attempts to locate JSON by finding the first '{' and last '}'.
    If that also failed,, return the text as-is.
    """
    text = text.strip()
    # 判断返回内容是不是被 Markdown 代码块包起来了，比如
    # ```json
    # {"facts": ["用户喜欢咖啡"]}
    # ```
    match = re.search(r"```(?:json)?\s*(.*?)\s*```", text, re.DOTALL)
    if match:
        # 如果找到了代码块，就把代码块内部内容取出来
        json_str = match.group(1)
    else:
        # 如果没找到代码块，就继续尝试从普通文本里抽 JSON
        start_idx = text.find("{")
        end_idx = text.rfind("}")
        if start_idx != -1 and end_idx != -1 and end_idx > start_idx:
            json_str = text[start_idx: end_idx + 1]
        else:
            json_str = text

    return json_str

def get_image_description(image_obj, llm, vision_details):
    """
    Get the description of the image

    Args:
        image_obj: 图片本身，可能是一个字符串 URL，也可能已经是现成的消息对象
        llm: 负责调用大模型的对象
        vision_details: 传给视觉模型的图片细节参数，比如高/低细节模式
    """

    if isinstance(image_obj, str):
        messages = [
            {
                "role": "user",
                "content": [
                    {
                        "type": "text",
                        "text": "A user is providing an image. Provide a high level description of the image and do not include any additional text."
                    },
                    {
                        "type": "image_url",
                        "image_url" : {
                            "url": image_obj,
                            "detail": vision_details,
                        }
                    }
                ]
            }
        ]
    else:
        # 说明调用方传进来的可能已经是一个格式化好的消息对象
        # 函数就不再包装，只是简单做成 messages = [image_obj]
        messages = [image_obj]

    response = llm.generate_response(messages=messages)
    return response

def parse_vision_messages(messages, llm=None, vision_details="auto"):
    """
    Parse the vision messages from the messages

    遍历一组聊天消息，把里面和图片相关的消息转换成“图片描述文本”，
    保留普通文本消息和 system 消息不变，最后返回一组“更适合纯文本 LLM 继续处理”的消息列表
    """
    returned_messages = []
    for msg in messages:
        # system 消息保留
        if msg["role"] == "system":
            returned_messages.append(msg)
            continue

        # content 是列表，按“多模态消息”处理，多模态协议里，这通常表示一条消息包含多个内容块，比如 一段文本、一张或多张图片、或多段混合内容
        if isinstance(msg["content"], list):
            # 不管有多少内容块，直接把整个让模型进行总结
            description = get_image_description(msg, llm, vision_details)
            returned_messages.append({"role": msg["role"], "content": description})

        # content 是单个图片对象
        elif isinstance(msg["content"], dict) and msg["content"].get("type") == "image_url":
            image_url = msg["content"]["image_url"]["url"]
            try:
                description = get_image_description(image_url, llm, vision_details)
                returned_messages.append({"role": msg["role"], "content": description})
            except Exception:
                raise Exception(f"Error while downloading {image_url}")
        else:
            returned_messages.append(msg)

    return returned_messages

def process_telemetry_filter(filters):
    """
    Process the telemetry filters

    对几个敏感身份字段做匿名化哈希处理
    """

    if filters is None:
        return {}

    encoded_ids = {}
    if "user_id" in filters:
        encoded_ids["user_id"] = hashlib.md5(filters["user_id"].encode()).hexdigest()
    if "agent_id" in filters:
        encoded_ids["agent_id"] = hashlib.md5(filters["agent_id"].encode()).hexdigest()
    if "run_id" in filters:
        encoded_ids["run_id"] = hashlib.md5(filters["run_id"].encode()).hexdigest()

    return list(filters.keys()), encoded_ids

def sanitize_relationship_for_cypher(relationship) -> str:
    """
    Sanitize relationship text for Cypher queeries by replacing problematic chatacters.

    把 relationship 字符串清洗成一个更适合放进 Cypher 查询里的关系名，主要是把各种特殊字符、标点符号替换成安全的文本片段，再把多余下划线压缩掉
    Cypher 是图数据库常用的一种查询语言，最典型的是 Neo4j
    """

    char_map = {
        "...": "_ellipsis_",
        "…": "_ellipsis_",
        "。": "_period_",
        "，": "_comma_",
        "；": "_semicolon_",
        "：": "_colon_",
        "！": "_exclamation_",
        "？": "_question_",
        "（": "_lparen_",
        "）": "_rparen_",
        "【": "_lbracket_",
        "】": "_rbracket_",
        "《": "_langle_",
        "》": "_rangle_",
        "'": "_apostrophe_",
        '"': "_quote_",
        "\\": "_backslash_",
        "/": "_slash_",
        "|": "_pipe_",
        "&": "_ampersand_",
        "=": "_equals_",
        "+": "_plus_",
        "*": "_asterisk_",
        "^": "_caret_",
        "%": "_percent_",
        "$": "_dollar_",
        "#": "_hash_",
        "@": "_at_",
        "!": "_bang_",
        "?": "_question_",
        "(": "_lparen_",
        ")": "_rparen_",
        "[": "_lbracket_",
        "]": "_rbracket_",
        "{": "_lbrace_",
        "}": "_rbrace_",
        "<": "_langle_",
        ">": "_rangle_",
        "-": "_",
    }

    sanitized = relationship
    for old, new in char_map.items():
        sanitized = sanitized.replace(old, new)

    return re.sub(r"_+", "_", sanitized).strip("_")

def remove_spaces_from_entities(
    entity_list: List[Any],
    *,
    sanitize_relationship: bool = True,
) -> List[Dict[str, Any]]:
    """
    Normalize entity relation dicts from LLM/tool output: lowercase, space to underscores.

    Skip entries that are not non-empty dicts or that lack any of
    ``source``, ``relationship``, or ``destination`` (avoid KeyError on ``[{}]`` or partial dicts)

    它期望输入如下所示：
    {
        "source": "Alice",
        "relationship": "Works At",
        "destination": "ByteDance"
    }

    Args:
        sanitize_relationship: 控制“关系名是否要按 Cypher 风格进一步清洗”
    """

    required = ("source", "relationship", "destination")
    cleaned: List[Dict[str, Any]] = []
    for item in entity_list:
        if not isinstance(item, dict) or not item:
            continue
        if not all(key in item for key in required):
            continue
        # 把 source、relationship、destination 转成小写并把空格替换成 _
        item["source"] = item["source"].lower().replace(" ", "_")
        rel = item["relationship"].lower().replace(" ", "_")
        # 按配置决定是否进一步清洗 relationship ，让它适合 Cypher 使用
        item["relationship"] = sanitize_relationship_for_cypher(rel) if sanitize_relationship else rel
        item["destination"] = item["destination"].lower().replace(" ", "_")
        cleaned.append(item)
    return cleaned
