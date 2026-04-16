import json
import hashlib
import mimetypes
import os
import re
import subprocess
import sys
import urllib.error
import urllib.request
from base64 import b64encode
from typing import Any, Optional


def get_declared_providers(env: Optional[dict[str, str]] = None) -> list[dict[str, Any]]:
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
    env: Optional[dict[str, str]] = None,
    http_post_json: Optional[Any] = None,
    http_get_json: Optional[Any] = None,
) -> dict[str, Any]:
    values = os.environ if env is None else env
    provider = values.get("IDEA_WORKER_PROVIDER", "placeholder").strip() or "placeholder"

    if provider == "placeholder":
        return build_placeholder_understanding(payload)

    request_func = http_post_json or post_json
    get_request_func = http_get_json or get_json
    try:
        if provider == "ollama":
            result = call_ollama(payload, values, request_func)
        elif provider == "lm_studio":
            result = call_lm_studio(payload, values, request_func, get_request_func)
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


def load_prompt_settings(env: Optional[dict[str, str]] = None) -> dict[str, Any]:
    values = os.environ if env is None else env
    path = str(values.get("IDEA_AI_PROMPT_SETTINGS_PATH", "tmp/ai-prompt-settings.json")).strip()
    if not path:
        return {}
    try:
        with open(path, "r", encoding="utf-8") as handle:
            payload = json.load(handle)
    except (OSError, json.JSONDecodeError):
        return {}
    return payload if isinstance(payload, dict) else {}


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
    env: Optional[dict[str, str]] = None,
    vector_reader: Optional[Any] = None,
    http_post_json: Optional[Any] = None,
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
    timeout = float(env.get("IDEA_WORKER_PROVIDER_TIMEOUT_SEC", "600"))
    body = {
        "model": model,
        "stream": False,
        "format": "json",
        "messages": [
            {
                "role": "user",
                "content": build_understanding_prompt(payload, env),
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


def call_lm_studio(payload: dict[str, Any], env: dict[str, str], http_post_json: Any, http_get_json: Any) -> dict[str, Any]:
    base_url = env.get("IDEA_WORKER_LM_STUDIO_URL", "http://127.0.0.1:1234").rstrip("/")
    configured_model = env.get("IDEA_WORKER_LM_STUDIO_MODEL", "qwen2.5-vl-7b").strip() or "qwen2.5-vl-7b"
    timeout = float(env.get("IDEA_WORKER_PROVIDER_TIMEOUT_SEC", "600"))
    loaded_models = list_lm_studio_models(base_url, timeout, http_get_json)
    model = resolve_lm_studio_model(configured_model, loaded_models)
    max_images = int(env.get("IDEA_WORKER_LM_STUDIO_MAX_IMAGES", "3") or 3)
    image_max_edge = int(env.get("IDEA_WORKER_LM_STUDIO_IMAGE_MAX_EDGE", "384") or 384)
    body = {
        "model": model,
        "messages": [
            {
                "role": "user",
                "content": build_lm_studio_content(payload, max_images=max_images, resize_max_edge=image_max_edge, env=env),
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
    message = choices[0].get("message") or {}
    content = extract_message_text_content(message)
    if not content:
        raise RuntimeError(f"lm studio response did not include usable message content: keys={sorted(message.keys())}")
    result = parse_json_object_text(content)
    result.setdefault("provider", "lm_studio")
    result.setdefault("model", response.get("model") or model)
    return result


def normalize_understanding_result(payload: dict[str, Any], provider: str) -> dict[str, Any]:
    raw_tags = sanitize_raw_tags(payload.get("raw_tags", []) or [])
    canonical_candidates = []
    for item in payload.get("canonical_candidates") or []:
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
    structured_attributes = payload.get("structured_attributes", {}) if isinstance(payload.get("structured_attributes", {}), dict) else {}
    if not canonical_candidates:
        canonical_candidates = derive_canonical_candidates(raw_tags, structured_attributes)

    return {
        "raw_tags": raw_tags,
        "canonical_candidates": canonical_candidates,
        "summary": str(payload.get("summary", "")).strip(),
        "sensitive_tags": [str(item).strip() for item in payload.get("sensitive_tags", []) if str(item).strip()],
        "quality_hints": [str(item).strip() for item in payload.get("quality_hints", []) if str(item).strip()],
        "structured_attributes": structured_attributes,
        "confidence": to_confidence(payload.get("confidence"), fallback=0.5),
        "provider": str(payload.get("provider", provider)).strip() or provider,
        "model": str(payload.get("model", "")).strip(),
        "raw_response": payload.get("raw_response", {}) if isinstance(payload.get("raw_response", {}), dict) else {},
    }


def build_understanding_prompt(payload: dict[str, Any], env: Optional[dict[str, str]] = None) -> str:
    media_type = str(payload.get("media_type", "unknown")).strip()
    file_path = str(payload.get("file_path", "")).strip()
    context = payload.get("context", {}) if isinstance(payload.get("context"), dict) else {}
    language = str(context.get("language", "zh-CN")).strip() or "zh-CN"
    max_tags = int(context.get("max_tags", 12) or 12)
    allow_sensitive_labels = bool(context.get("allow_sensitive_labels", False))
    prompt = (
        "你是本地离线媒体整理助手。"
        "请只返回一个 JSON 对象，字段必须包含 "
        "raw_tags, canonical_candidates, summary, sensitive_tags, quality_hints, structured_attributes, confidence。"
        "不要输出 markdown，不要解释。"
        "canonical_candidates 只允许使用以下 namespace：content, quality, sensitive, person, management。"
        "content 用于内容和场景语义；quality 用于清晰度、压缩、分辨率等；"
        "sensitive 用于敏感内容标签；person 用于可重复识别的人物外观候选标签，但不要编造真实身份姓名；"
        "management 仅用于明显的治理状态建议。"
        "sensitive 标签必须具体到可观察的行为、暴露部位、道具或情境，例如 口交、阴道性交、肛交、自慰、乳房暴露、外阴暴露、颜射、束缚。"
        "不要输出 敏感内容、成人内容、露骨、NSFW、做爱场景 这类过于宽泛的标签。"
        "content 标签也不要只写 图片、视频、人物 这类信息量过低的泛标签，优先写更具体的场景或服饰，如 JK制服、酒店床上场景、暗调特写。"
        "如果画面里人物具有可重复辨识的外观特征，可以输出 1-3 个 person 标签，"
        "例如 长发女性候选、短发男性候选、纹身男性候选、双人组合候选。"
        "structured_attributes 尽量包含：media_type, subject_count, capture_type, orientation, has_face, is_sensitive。"
        "summary 要简洁直白，避免含糊表达。"
        "所有 summary、raw_tags、canonical_candidates.name，以及 structured_attributes 里的字符串值，默认都使用简体中文。"
        "不要输出英文标签，不要把文件路径、目录名、盘符路径、URL 当成标签或摘要内容。"
        "canonical_candidates 必须返回 1-5 个有效候选，不能为 null。"
        f" language={language}; max_tags={max_tags}; media_type={media_type}; file_path={file_path};"
        f" allow_sensitive_labels={str(allow_sensitive_labels).lower()}."
    )
    extra_prompt = str(load_prompt_settings(env).get("understanding_extra_prompt", "")).strip()
    if extra_prompt:
        prompt = f"{prompt} 额外规则：{extra_prompt}"
    return prompt


def build_image_inputs(
    payload: dict[str, Any],
    max_images: Optional[int] = None,
    resize_max_edge: Optional[int] = None,
) -> list[str]:
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
    limit = 6 if max_images is None or max_images <= 0 else max_images
    for path in image_paths[:limit]:
        encoded_item = encode_file_base64(path, resize_max_edge=resize_max_edge)
        if encoded_item:
            encoded.append(encoded_item)
    return encoded


def build_lm_studio_content(
    payload: dict[str, Any],
    max_images: Optional[int] = None,
    resize_max_edge: Optional[int] = None,
    env: Optional[dict[str, str]] = None,
) -> list[dict[str, Any]]:
    content = [{"type": "text", "text": build_understanding_prompt(payload, env=env)}]
    for encoded_item in build_image_inputs(payload, max_images=max_images, resize_max_edge=resize_max_edge):
        content.append({
            "type": "image_url",
            "image_url": {"url": encoded_item},
        })
    return content


def encode_file_base64(path: str, resize_max_edge: Optional[int] = None) -> str:
    try:
        raw, mime_type = read_file_for_base64(path, resize_max_edge=resize_max_edge)
    except OSError:
        return ""
    if not mime_type:
        mime_type = "application/octet-stream"
    return f"data:{mime_type};base64,{b64encode(raw).decode('ascii')}"


def read_file_for_base64(path: str, resize_max_edge: Optional[int] = None) -> tuple[bytes, str]:
    if resize_max_edge is not None and resize_max_edge > 0:
        process = subprocess.run(
            [
                os.environ.get("IDEA_WORKER_FFMPEG_BIN", "ffmpeg"),
                "-v",
                "error",
                "-i",
                path,
                "-vf",
                f"scale='if(gt(iw,ih),min({resize_max_edge},iw),-2)':'if(gt(iw,ih),-2,min({resize_max_edge},ih))'",
                "-frames:v",
                "1",
                "-q:v",
                "8",
                "-f",
                "image2pipe",
                "-vcodec",
                "mjpeg",
                "-",
            ],
            check=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        if process.returncode == 0 and process.stdout:
            return process.stdout, "image/jpeg"

    with open(path, "rb") as handle:
        raw = handle.read()
    mime_type, _ = mimetypes.guess_type(path)
    return raw, mime_type or "application/octet-stream"


def get_json(url: str, timeout: float) -> dict[str, Any]:
    request = urllib.request.Request(url, method="GET")
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"http {exc.code}: {detail}") from exc
    except urllib.error.URLError as exc:
        raise RuntimeError(f"provider unavailable: {exc.reason}") from exc


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


def list_lm_studio_models(base_url: str, timeout: float, http_get_json: Any) -> list[str]:
    response = http_get_json(f"{base_url}/v1/models", timeout)
    items = response.get("data") or []
    models = []
    for item in items:
        if not isinstance(item, dict):
            continue
        model_id = str(item.get("id", "")).strip()
        if model_id:
            models.append(model_id)
    return models


def resolve_lm_studio_model(configured_model: str, loaded_models: list[str]) -> str:
    if configured_model and configured_model in loaded_models:
        return configured_model
    for model in loaded_models:
        if not is_probably_embedding_model(model):
            return model
    for model in loaded_models:
        if is_probably_vision_model(model):
            return model
    loaded_text = ", ".join(loaded_models) if loaded_models else "(none)"
    raise RuntimeError(
        f"configured lm_studio model {configured_model} is not loaded and no usable chat model is available; loaded_models={loaded_text}"
    )


def is_probably_vision_model(model_name: str) -> bool:
    value = str(model_name or "").strip().lower()
    return any(token in value for token in (
        "vl",
        "vision",
        "llava",
        "minicpm-v",
        "internvl",
        "glm-4v",
        "qvq",
        "phi-3.5-vision",
    ))


def is_probably_embedding_model(model_name: str) -> bool:
    value = str(model_name or "").strip().lower()
    return "embed" in value or "embedding" in value


def extract_message_text_content(message: dict[str, Any]) -> str:
    content = message.get("content")
    if isinstance(content, str):
        return content.strip()
    if isinstance(content, list):
        parts = []
        for item in content:
            if not isinstance(item, dict):
                continue
            if str(item.get("type", "")).strip() == "text":
                text = str(item.get("text", "")).strip()
                if text:
                    parts.append(text)
        return "\n".join(parts).strip()
    if isinstance(content, dict):
        text = str(content.get("text", "")).strip()
        if text:
            return text
    return ""


def parse_json_object_text(text: str) -> dict[str, Any]:
    candidate = text.strip()
    if candidate.startswith("```"):
        candidate = re.sub(r"^```[a-zA-Z0-9_-]*\s*", "", candidate)
        candidate = re.sub(r"\s*```$", "", candidate)
        candidate = candidate.strip()
    try:
        return json.loads(candidate)
    except json.JSONDecodeError:
        start = candidate.find("{")
        end = candidate.rfind("}")
        if start >= 0 and end > start:
            return json.loads(candidate[start:end + 1])
        raise


def sanitize_raw_tags(items: list[Any]) -> list[str]:
    result = []
    for item in items:
        text = str(item).strip()
        if not text:
            continue
        if looks_like_path(text):
            continue
        result.append(text)
    return result


def looks_like_path(value: str) -> bool:
    text = value.strip()
    if not text:
        return False
    lowered = text.lower()
    if "://" in lowered:
        return True
    if "/" in text or "\\" in text:
        return True
    return len(text) > 2 and text[1:3] == ":\\"


def derive_canonical_candidates(raw_tags: list[str], structured_attributes: dict[str, Any]) -> list[dict[str, Any]]:
    candidates: list[dict[str, Any]] = []
    seen: set[str] = set()

    def add(namespace: str, name: str, confidence: float) -> None:
        namespace = str(namespace).strip()
        name = str(name).strip()
        if not namespace or not name:
            return
        key = f"{namespace}:{name}"
        if key in seen:
            return
        seen.add(key)
        candidates.append({
            "namespace": namespace,
            "name": name,
            "confidence": confidence,
        })

    subject_count = string_value(structured_attributes.get("subject_count")).lower()
    if subject_count in ("1", "single", "one"):
        add("content", "单人", 0.65)
    elif subject_count in ("2", "two", "couple"):
        add("content", "双人", 0.65)
    elif subject_count in ("multiple", "many", "group"):
        add("content", "多人", 0.65)

    capture_type = string_value(structured_attributes.get("capture_type")).lower()
    if capture_type in ("closeup", "close_up", "close-up"):
        add("content", "局部特写", 0.6)
    elif capture_type in ("selfie",):
        add("content", "自拍", 0.7)

    if truthy_value(structured_attributes.get("has_face")):
        add("person", "露脸", 0.6)

    for tag in raw_tags:
        normalized = normalize_tag_candidate_name(tag)
        if not normalized:
            continue
        namespace = infer_candidate_namespace(normalized)
        add(namespace, normalized, 0.6)

    return candidates[:5]


def normalize_tag_candidate_name(tag: str) -> str:
    value = str(tag).strip()
    lowered = value.lower()
    mapping = {
        "jk": "JK制服",
        "glasses": "眼镜",
        "straps": "束缚",
        "bondage": "束缚",
        "darkmode": "低光高对比",
        "dark mode": "低光高对比",
        "closeup": "局部特写",
        "close-up": "局部特写",
        "close_up": "局部特写",
        "blowjob": "口交",
        "oral": "口交",
        "oral sex": "口交",
        "cunnilingus": "舔阴",
        "vaginal": "阴道性交",
        "vaginal sex": "阴道性交",
        "anal": "肛交",
        "anal sex": "肛交",
        "masturbation": "自慰",
        "handjob": "手淫",
        "cumshot": "射精镜头",
        "facial": "颜射",
        "pussy": "外阴暴露",
        "vulva": "外阴暴露",
        "breasts": "乳房暴露",
        "boobs": "乳房暴露",
        "lingerie": "内衣",
        "uniform": "制服",
        "video": "",
        "image": "",
    }
    if lowered in mapping:
        return mapping[lowered]
    if looks_like_path(value):
        return ""
    return value


def infer_candidate_namespace(name: str) -> str:
    if name in ("口交", "舔阴", "阴道性交", "肛交", "自慰", "手淫", "乳房暴露", "外阴暴露", "颜射", "射精镜头", "束缚"):
        return "sensitive"
    if name in ("低光高对比", "低照度", "模糊", "清晰"):
        return "quality"
    if name in ("眼镜", "露脸"):
        return "person"
    return "content"


def string_value(value: Any) -> str:
    if value is None:
        return ""
    if isinstance(value, str):
        return value.strip()
    return str(value).strip()


def truthy_value(value: Any) -> bool:
    if isinstance(value, bool):
        return value
    text = string_value(value).lower()
    return text in ("true", "1", "yes", "on")


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
