import json
import hashlib
import mimetypes
import os
import subprocess
import sys
import urllib.error
import urllib.request
from base64 import b64encode
from typing import Any


def get_declared_providers(env: dict[str, str] | None = None) -> list[dict[str, Any]]:
    values = os.environ if env is None else env
    return [
        {
            "name": "ollama",
            "enabled": True,
            "default_model": values.get("IDEA_WORKER_OLLAMA_MODEL", "qwen3-vl-8b"),
        },
        {
            "name": "lm_studio",
            "enabled": True,
            "default_model": values.get("IDEA_WORKER_LM_STUDIO_MODEL", "qwen2.5-vl-7b"),
        },
        {
            "name": "placeholder",
            "enabled": True,
            "default_model": "placeholder-v1",
        },
    ]


def build_placeholder_understanding(payload: dict[str, Any]) -> dict[str, Any]:
    media_type = str(payload.get("media_type", "")).strip().lower()
    file_path = str(payload.get("file_path", "")).strip()
    file_name = file_path.split("/")[-1] if file_path else ""

    if media_type == "video":
        raw_tags = ["视频", "待AI精标"]
        canonical_candidates = [
            {"namespace": "content", "name": "视频", "confidence": 0.70},
            {"namespace": "management", "name": "待AI精标", "confidence": 0.60},
        ]
        summary = f"{file_name or '未命名文件'}，视频文件，当前为占位理解结果。"
    else:
        raw_tags = ["图片", "待AI精标"]
        canonical_candidates = [
            {"namespace": "content", "name": "图片", "confidence": 0.70},
            {"namespace": "management", "name": "待AI精标", "confidence": 0.60},
        ]
        summary = f"{file_name or '未命名文件'}，图片文件，当前为占位理解结果。"

    return {
        "raw_tags": raw_tags,
        "canonical_candidates": canonical_candidates,
        "summary": summary,
        "sensitive_tags": [],
        "quality_hints": ["placeholder"],
        "structured_attributes": {
            "media_type": media_type or "unknown",
            "is_sensitive": False,
        },
        "confidence": 0.55,
        "provider": "placeholder",
        "model": "placeholder-v1",
        "raw_response": {
            "mode": "placeholder",
        },
    }


def understand_media(
    payload: dict[str, Any],
    env: dict[str, str] | None = None,
    http_post_json: Any | None = None,
) -> dict[str, Any]:
    values = os.environ if env is None else env
    provider = values.get("IDEA_WORKER_PROVIDER", "placeholder").strip() or "placeholder"

    if provider == "placeholder":
        return build_placeholder_understanding(payload)

    request_func = http_post_json or post_json
    try:
        if provider == "ollama":
            result = call_ollama(payload, values, request_func)
        elif provider == "lm_studio":
            result = call_lm_studio(payload, values, request_func)
        else:
            raise RuntimeError(f"unsupported provider: {provider}")
        return normalize_understanding_result(result, provider=provider)
    except Exception as exc:
        fallback = build_placeholder_understanding(payload)
        fallback["raw_response"] = {
            "mode": "fallback_placeholder",
            "requested_provider": provider,
            "fallback_reason": str(exc),
        }
        return fallback


def phash_to_vector(phash: str) -> str:
    values = []
    for char in str(phash or "").strip().lower():
        try:
            nibble = int(char, 16)
        except ValueError:
            continue
        for shift in (3, 2, 1, 0):
            values.append("1.0000" if ((nibble >> shift) & 1) == 1 else "0.0000")
    if not values:
        return ""
    return "[" + ",".join(values) + "]"


def build_placeholder_embeddings(payload: dict[str, Any]) -> dict[str, Any]:
    image_vector = phash_to_vector(str(payload.get("image_phash", "")).strip())
    frame_vectors = []
    for item in payload.get("frames", []) or []:
        if not isinstance(item, dict):
            continue
        frame_id = int(item.get("frame_id", 0) or 0)
        vector = phash_to_vector(str(item.get("phash", "")).strip())
        if frame_id <= 0 or not vector:
            continue
        frame_vectors.append({
            "frame_id": frame_id,
            "vector": vector,
        })

    return {
        "vector": image_vector,
        "frame_vectors": frame_vectors,
        "provider": "placeholder",
        "model": "placeholder-v1",
        "raw_response": {
            "mode": "placeholder",
            "embedding_type": str(payload.get("embedding_type", "")).strip(),
        },
    }


def read_visual_vector(path: str, ffmpeg_bin: str = "ffmpeg") -> str:
    if not path:
        return ""
    process = subprocess.run(
        [
            ffmpeg_bin,
            "-v",
            "error",
            "-i",
            path,
            "-vf",
            "scale=8:8,format=gray",
            "-frames:v",
            "1",
            "-f",
            "rawvideo",
            "-",
        ],
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    raw = process.stdout
    if not raw:
        return ""
    values = [f"{byte / 255.0:.4f}" for byte in raw[:64]]
    if not values:
        return ""
    return "[" + ",".join(values) + "]"


def build_pixel_embeddings(
    payload: dict[str, Any],
    env: dict[str, str],
    vector_reader: Any,
) -> dict[str, Any]:
    ffmpeg_bin = env.get("IDEA_WORKER_FFMPEG_BIN", "ffmpeg")
    image_vector = vector_reader(str(payload.get("file_path", "")).strip(), ffmpeg_bin)
    frame_vectors = []
    for item in payload.get("frames", []) or []:
        if not isinstance(item, dict):
            continue
        frame_id = int(item.get("frame_id", 0) or 0)
        frame_path = str(item.get("frame_path", "")).strip()
        vector = vector_reader(frame_path, ffmpeg_bin)
        if frame_id <= 0 or not vector:
            continue
        frame_vectors.append({
            "frame_id": frame_id,
            "vector": vector,
        })

    return {
        "vector": image_vector,
        "frame_vectors": frame_vectors,
        "provider": "pixel",
        "model": "ffmpeg-gray-8x8-v1",
        "raw_response": {
            "mode": "pixel",
            "embedding_type": str(payload.get("embedding_type", "")).strip(),
        },
    }


def build_semantic_tokens(understanding: dict[str, Any]) -> list[str]:
    tokens = []
    for item in understanding.get("raw_tags", []) or []:
        text = str(item).strip()
        if text:
            tokens.append(f"raw:{text}")
    for item in understanding.get("canonical_candidates", []) or []:
        if not isinstance(item, dict):
            continue
        namespace = str(item.get("namespace", "")).strip()
        name = str(item.get("name", "")).strip()
        if namespace and name:
            tokens.append(f"canonical:{namespace}:{name}")
    for item in understanding.get("sensitive_tags", []) or []:
        text = str(item).strip()
        if text:
            tokens.append(f"sensitive:{text}")
    for item in understanding.get("quality_hints", []) or []:
        text = str(item).strip()
        if text:
            tokens.append(f"quality:{text}")
    attrs = understanding.get("structured_attributes", {})
    if isinstance(attrs, dict):
        for key in sorted(attrs.keys()):
            value = attrs.get(key)
            text = str(value).strip()
            if text:
                tokens.append(f"attr:{key}={text}")
    summary = str(understanding.get("summary", "")).strip()
    if summary:
        tokens.append(f"summary:{summary}")
    return tokens


def semantic_vector_from_tokens(tokens: list[str], dimensions: int = 64) -> str:
    if not tokens or dimensions <= 0:
        return ""
    buckets = [0.0] * dimensions
    for token in tokens:
        digest = hashlib.sha1(token.encode("utf-8")).digest()
        index = digest[0] % dimensions
        buckets[index] += 1.0
    peak = max(buckets)
    if peak <= 0:
        return ""
    normalized = [f"{value / peak:.4f}" for value in buckets]
    return "[" + ",".join(normalized) + "]"


def build_semantic_embeddings(
    payload: dict[str, Any],
    env: dict[str, str],
    http_post_json: Any,
) -> dict[str, Any]:
    context = payload.get("context", {}) if isinstance(payload.get("context"), dict) else {}
    image_payload = {
        "media_type": str(payload.get("media_type", "image")).strip() or "image",
        "file_path": str(payload.get("file_path", "")).strip(),
        "frame_paths": [],
        "context": context,
    }
    image_understanding = understand_media(image_payload, env=env, http_post_json=http_post_json)
    image_vector = semantic_vector_from_tokens(build_semantic_tokens(image_understanding))

    frame_vectors = []
    for item in payload.get("frames", []) or []:
        if not isinstance(item, dict):
            continue
        frame_id = int(item.get("frame_id", 0) or 0)
        frame_path = str(item.get("frame_path", "")).strip()
        if frame_id <= 0 or not frame_path:
            continue
        frame_understanding = understand_media(
            {
                "media_type": "image",
                "file_path": frame_path,
                "frame_paths": [],
                "context": context,
            },
            env=env,
            http_post_json=http_post_json,
        )
        frame_vector = semantic_vector_from_tokens(build_semantic_tokens(frame_understanding))
        if not frame_vector:
            continue
        frame_vectors.append({
            "frame_id": frame_id,
            "vector": frame_vector,
        })

    provider_name = str(image_understanding.get("provider", env.get("IDEA_WORKER_PROVIDER", "unknown"))).strip() or "unknown"
    model_name = str(image_understanding.get("model", "")).strip() or "unknown"
    return {
        "vector": image_vector,
        "frame_vectors": frame_vectors,
        "provider": "semantic",
        "model": f"semantic-{provider_name}-{model_name}-v1",
        "raw_response": {
            "mode": "semantic",
            "underlying_provider": provider_name,
            "underlying_model": model_name,
            "embedding_type": str(payload.get("embedding_type", "")).strip(),
        },
    }


def embed_media(
    payload: dict[str, Any],
    env: dict[str, str] | None = None,
    vector_reader: Any | None = None,
    http_post_json: Any | None = None,
) -> dict[str, Any]:
    values = os.environ if env is None else env
    provider = values.get("IDEA_WORKER_EMBED_PROVIDER", "pixel").strip() or "pixel"
    reader = vector_reader or read_visual_vector
    request_func = http_post_json or post_json
    if provider == "pixel":
        try:
            return build_pixel_embeddings(payload, values, reader)
        except Exception as exc:
            fallback = build_placeholder_embeddings(payload)
            fallback["raw_response"] = {
                "mode": "fallback_placeholder",
                "requested_provider": provider,
                "fallback_reason": str(exc),
            }
            return fallback
    if provider == "semantic":
        try:
            return build_semantic_embeddings(payload, values, request_func)
        except Exception as exc:
            fallback = build_placeholder_embeddings(payload)
            fallback["raw_response"] = {
                "mode": "fallback_placeholder",
                "requested_provider": provider,
                "fallback_reason": str(exc),
            }
            return fallback
    if provider == "placeholder":
        return build_placeholder_embeddings(payload)

    fallback = build_placeholder_embeddings(payload)
    fallback["raw_response"] = {
        "mode": "fallback_placeholder",
        "requested_provider": provider,
        "fallback_reason": "embed_media provider not implemented",
    }
    return fallback


def call_ollama(payload: dict[str, Any], env: dict[str, str], http_post_json: Any) -> dict[str, Any]:
    base_url = env.get("IDEA_WORKER_OLLAMA_URL", "http://127.0.0.1:11434").rstrip("/")
    model = env.get("IDEA_WORKER_OLLAMA_MODEL", "qwen3-vl-8b")
    timeout = float(env.get("IDEA_WORKER_PROVIDER_TIMEOUT_SEC", "60"))
    body = {
        "model": model,
        "stream": False,
        "format": "json",
        "messages": [
            {
                "role": "user",
                "content": build_understanding_prompt(payload),
                "images": build_image_inputs(payload),
            }
        ],
    }
    response = http_post_json(
        f"{base_url}/api/chat",
        body,
        {"Content-Type": "application/json"},
        timeout,
    )
    content = (((response.get("message") or {}).get("content")) or "").strip()
    if not content:
        raise RuntimeError("ollama response did not include message.content")
    result = json.loads(content)
    result.setdefault("provider", "ollama")
    result.setdefault("model", response.get("model") or model)
    return result


def call_lm_studio(payload: dict[str, Any], env: dict[str, str], http_post_json: Any) -> dict[str, Any]:
    base_url = env.get("IDEA_WORKER_LM_STUDIO_URL", "http://127.0.0.1:1234").rstrip("/")
    model = env.get("IDEA_WORKER_LM_STUDIO_MODEL", "qwen2.5-vl-7b")
    timeout = float(env.get("IDEA_WORKER_PROVIDER_TIMEOUT_SEC", "60"))
    body = {
        "model": model,
        "response_format": {"type": "json_object"},
        "messages": [
            {
                "role": "user",
                "content": build_lm_studio_content(payload),
            }
        ],
    }
    response = http_post_json(
        f"{base_url}/v1/chat/completions",
        body,
        {"Content-Type": "application/json"},
        timeout,
    )
    choices = response.get("choices") or []
    if not choices:
        raise RuntimeError("lm studio response did not include choices")
    message = (choices[0].get("message") or {}).get("content")
    if not isinstance(message, str) or not message.strip():
        raise RuntimeError("lm studio response did not include message content")
    result = json.loads(message)
    result.setdefault("provider", "lm_studio")
    result.setdefault("model", model)
    return result


def normalize_understanding_result(payload: dict[str, Any], provider: str) -> dict[str, Any]:
    raw_tags = [str(item).strip() for item in payload.get("raw_tags", []) if str(item).strip()]
    canonical_candidates = []
    for item in payload.get("canonical_candidates", []):
        if not isinstance(item, dict):
            continue
        namespace = str(item.get("namespace", "")).strip()
        name = str(item.get("name", "")).strip()
        if not namespace or not name:
            continue
        canonical_candidates.append({
            "namespace": namespace,
            "name": name,
            "confidence": to_confidence(item.get("confidence")),
        })

    return {
        "raw_tags": raw_tags,
        "canonical_candidates": canonical_candidates,
        "summary": str(payload.get("summary", "")).strip(),
        "sensitive_tags": [str(item).strip() for item in payload.get("sensitive_tags", []) if str(item).strip()],
        "quality_hints": [str(item).strip() for item in payload.get("quality_hints", []) if str(item).strip()],
        "structured_attributes": payload.get("structured_attributes", {}) if isinstance(payload.get("structured_attributes", {}), dict) else {},
        "confidence": to_confidence(payload.get("confidence"), fallback=0.5),
        "provider": str(payload.get("provider", provider)).strip() or provider,
        "model": str(payload.get("model", "")).strip(),
        "raw_response": payload.get("raw_response", {}) if isinstance(payload.get("raw_response", {}), dict) else {},
    }


def build_understanding_prompt(payload: dict[str, Any]) -> str:
    media_type = str(payload.get("media_type", "unknown")).strip()
    file_path = str(payload.get("file_path", "")).strip()
    context = payload.get("context", {}) if isinstance(payload.get("context"), dict) else {}
    language = str(context.get("language", "zh-CN")).strip() or "zh-CN"
    max_tags = int(context.get("max_tags", 12) or 12)
    allow_sensitive_labels = bool(context.get("allow_sensitive_labels", False))
    return (
        "你是本地离线媒体整理助手。"
        "请只返回一个 JSON 对象，字段必须包含 "
        "raw_tags, canonical_candidates, summary, sensitive_tags, quality_hints, structured_attributes, confidence。"
        "不要输出 markdown，不要解释。"
        "canonical_candidates 只允许使用以下 namespace：content, quality, sensitive, person, management。"
        "content 用于内容和场景语义；quality 用于清晰度、压缩、分辨率等；"
        "sensitive 用于敏感内容标签；person 用于可重复识别的人物外观候选标签，但不要编造真实身份姓名；"
        "management 仅用于明显的治理状态建议。"
        "如果画面里人物具有可重复辨识的外观特征，可以输出 1-3 个 person 标签，"
        "例如 长发女性候选、短发男性候选、纹身男性候选、双人组合候选。"
        "structured_attributes 尽量包含：media_type, subject_count, capture_type, orientation, has_face, is_sensitive。"
        "summary 要简洁直白，避免含糊表达。"
        f" language={language}; max_tags={max_tags}; media_type={media_type}; file_path={file_path};"
        f" allow_sensitive_labels={str(allow_sensitive_labels).lower()}."
    )


def build_image_inputs(payload: dict[str, Any]) -> list[str]:
    image_paths = []
    media_type = str(payload.get("media_type", "")).strip().lower()
    file_path = str(payload.get("file_path", "")).strip()
    if media_type == "image" and file_path:
        image_paths.append(file_path)
    for path in payload.get("frame_paths", []) or []:
        text = str(path).strip()
        if text:
            image_paths.append(text)

    encoded = []
    for path in image_paths[:6]:
        encoded_item = encode_file_base64(path)
        if encoded_item:
            encoded.append(encoded_item)
    return encoded


def build_lm_studio_content(payload: dict[str, Any]) -> list[dict[str, Any]]:
    content = [{"type": "text", "text": build_understanding_prompt(payload)}]
    for encoded_item in build_image_inputs(payload):
        content.append({
            "type": "image_url",
            "image_url": {"url": encoded_item},
        })
    return content


def encode_file_base64(path: str) -> str:
    try:
        with open(path, "rb") as handle:
            raw = handle.read()
    except OSError:
        return ""
    mime_type, _ = mimetypes.guess_type(path)
    if not mime_type:
        mime_type = "application/octet-stream"
    return f"data:{mime_type};base64,{b64encode(raw).decode('ascii')}"


def post_json(url: str, body: dict[str, Any], headers: dict[str, str], timeout: float) -> dict[str, Any]:
    request = urllib.request.Request(
        url,
        data=json.dumps(body).encode("utf-8"),
        headers=headers,
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"http {exc.code}: {detail}") from exc
    except urllib.error.URLError as exc:
        raise RuntimeError(f"provider unavailable: {exc.reason}") from exc


def to_confidence(value: Any, fallback: float = 0.0) -> float:
    try:
        score = float(value)
    except (TypeError, ValueError):
        return fallback
    if score < 0:
        return 0.0
    if score > 1:
        return 1.0
    return score


def handle_message(message: dict[str, Any]) -> dict[str, Any]:
    request_id = message.get("request_id", "")
    message_type = message.get("type", "")

    if message_type == "health_check":
        return {
            "request_id": request_id,
            "type": "result",
            "ok": True,
            "payload": {
                "status": "ok",
                "providers": get_declared_providers(),
            },
        }

    if message_type == "list_models":
        return {
            "request_id": request_id,
            "type": "result",
            "ok": True,
            "payload": {
                "providers": get_declared_providers(),
            },
        }

    if message_type == "understand_media":
        return {
            "request_id": request_id,
            "type": "result",
            "ok": True,
            "payload": understand_media(message.get("payload", {})),
        }

    if message_type == "embed_media":
        return {
            "request_id": request_id,
            "type": "result",
            "ok": True,
            "payload": embed_media(message.get("payload", {})),
        }

    return {
        "request_id": request_id,
        "type": "error",
        "ok": False,
        "error": {
            "code": "unsupported_message_type",
            "message": f"unsupported message type: {message_type}",
            "retryable": False,
        },
    }


def main() -> int:
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue

        try:
            message = json.loads(line)
        except json.JSONDecodeError as exc:
            response = {
                "request_id": "",
                "type": "error",
                "ok": False,
                "error": {
                    "code": "invalid_json",
                    "message": str(exc),
                    "retryable": False,
                },
            }
            sys.stdout.write(json.dumps(response) + "\n")
            sys.stdout.flush()
            continue

        response = handle_message(message)
        sys.stdout.write(json.dumps(response) + "\n")
        sys.stdout.flush()

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
