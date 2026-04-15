import json
import unittest
from unittest import mock

from worker.main import (
    build_understanding_prompt,
    normalize_understanding_result,
    embed_media,
    handle_message,
    parse_json_object_text,
    understand_media,
)


def vector_dims(value: str) -> int:
    text = value.strip().strip("[]")
    if not text:
        return 0
    return len([part for part in text.split(",") if part.strip()])


class WorkerProtocolTest(unittest.TestCase):
    def test_health_check_returns_ok(self):
        response = handle_message({
            "request_id": "req-1",
            "type": "health_check",
        })

        self.assertEqual("req-1", response["request_id"])
        self.assertEqual("result", response["type"])
        self.assertTrue(response["ok"])
        self.assertEqual("ok", response["payload"]["status"])

    def test_list_models_returns_declared_models(self):
        response = handle_message({
            "request_id": "req-2",
            "type": "list_models",
        })

        self.assertEqual("req-2", response["request_id"])
        self.assertTrue(response["ok"])
        self.assertIn("providers", response["payload"])

    def test_understand_media_returns_placeholder_understanding(self):
        response = handle_message({
            "request_id": "req-3",
            "type": "understand_media",
            "payload": {
                "file_id": 9,
                "media_type": "image",
                "file_path": "/Volumes/media/photos/poster.jpg",
                "frame_paths": [],
                "context": {
                    "allow_sensitive_labels": True,
                    "max_tags": 8,
                    "language": "zh-CN",
                },
            },
        })

        self.assertEqual("req-3", response["request_id"])
        self.assertEqual("result", response["type"])
        self.assertTrue(response["ok"])
        self.assertIn("raw_tags", response["payload"])
        self.assertIn("canonical_candidates", response["payload"])
        self.assertIn("summary", response["payload"])
        self.assertEqual("placeholder", response["payload"]["provider"])

    def test_understand_media_uses_ollama_when_configured(self):
        def fake_http_post_json(url, body, headers, timeout):
            self.assertEqual("http://127.0.0.1:11434/api/chat", url)
            self.assertEqual("qwen3-vl-8b", body["model"])
            self.assertFalse(body["stream"])
            self.assertIn("canonical_candidates 只允许使用以下 namespace", body["messages"][0]["content"])
            self.assertIn("person 用于可重复识别的人物外观候选标签", body["messages"][0]["content"])
            return {
                "model": "qwen3-vl-8b",
                "message": {
                    "content": """
                    {
                      "raw_tags": ["单人写真", "室内"],
                      "canonical_candidates": [{"namespace":"content","name":"单人写真","confidence":0.93}],
                      "summary": "单人室内写真，画面清晰。",
                      "sensitive_tags": [],
                      "quality_hints": ["清晰度高"],
                      "structured_attributes": {"media_type":"image","orientation":"portrait"},
                      "confidence": 0.88
                    }
                    """,
                },
            }

        payload = understand_media(
            {
                "file_id": 9,
                "media_type": "image",
                "file_path": "/Volumes/media/photos/poster.jpg",
                "frame_paths": [],
                "context": {"language": "zh-CN", "max_tags": 8, "allow_sensitive_labels": True},
            },
            env={
                "IDEA_WORKER_PROVIDER": "ollama",
                "IDEA_WORKER_OLLAMA_URL": "http://127.0.0.1:11434",
                "IDEA_WORKER_OLLAMA_MODEL": "qwen3-vl-8b",
            },
            http_post_json=fake_http_post_json,
        )

        self.assertEqual("ollama", payload["provider"])
        self.assertEqual("qwen3-vl-8b", payload["model"])
        self.assertEqual("单人室内写真，画面清晰。", payload["summary"])
        self.assertEqual("单人写真", payload["canonical_candidates"][0]["name"])

    def test_understand_media_falls_back_to_placeholder_when_provider_fails(self):
        payload = understand_media(
            {
                "file_id": 10,
                "media_type": "video",
                "file_path": "/Volumes/media/video/clip.mp4",
                "frame_paths": [],
                "context": {"language": "zh-CN", "max_tags": 8, "allow_sensitive_labels": True},
            },
            env={"IDEA_WORKER_PROVIDER": "ollama"},
            http_post_json=mock.Mock(side_effect=RuntimeError("provider down")),
        )

        self.assertEqual("placeholder", payload["provider"])
        self.assertEqual("fallback_placeholder", payload["raw_response"]["mode"])
        self.assertEqual("ollama", payload["raw_response"]["requested_provider"])
        self.assertIn("provider down", payload["raw_response"]["fallback_reason"])

    def test_understand_media_uses_lm_studio_with_loaded_vision_model(self):
        def fake_http_get_json(url, timeout):
            self.assertEqual("http://127.0.0.1:1234/v1/models", url)
            return {
                "data": [
                    {"id": "qwen2.5-vl-7b"},
                    {"id": "text-embedding-nomic-embed-text-v1.5"},
                ]
            }

        def fake_http_post_json(url, body, headers, timeout):
            self.assertEqual("http://127.0.0.1:1234/v1/chat/completions", url)
            self.assertEqual("qwen2.5-vl-7b", body["model"])
            self.assertNotIn("response_format", body)
            return {
                "model": "qwen2.5-vl-7b",
                "choices": [
                    {
                        "message": {
                            "content": """
                            ```json
                            {
                              "raw_tags": ["视频", "室内"],
                              "canonical_candidates": [{"namespace":"content","name":"视频","confidence":0.91}],
                              "summary": "室内视频画面。",
                              "sensitive_tags": [],
                              "quality_hints": ["清晰"],
                              "structured_attributes": {"media_type":"video"},
                              "confidence": 0.87
                            }
                            ```
                            """
                        }
                    }
                ]
            }

        payload = understand_media(
            {
                "file_id": 11,
                "media_type": "video",
                "file_path": "/Volumes/media/video/clip.mp4",
                "frame_paths": ["/tmp/frame-1.jpg"],
                "context": {"language": "zh-CN", "max_tags": 8, "allow_sensitive_labels": True},
            },
            env={
                "IDEA_WORKER_PROVIDER": "lm_studio",
                "IDEA_WORKER_LM_STUDIO_URL": "http://127.0.0.1:1234",
                "IDEA_WORKER_LM_STUDIO_MODEL": "qwen2.5-vl-7b",
            },
            http_post_json=fake_http_post_json,
            http_get_json=fake_http_get_json,
        )

        self.assertEqual("lm_studio", payload["provider"])
        self.assertEqual("qwen2.5-vl-7b", payload["model"])
        self.assertEqual("室内视频画面。", payload["summary"])
        self.assertEqual("视频", payload["canonical_candidates"][0]["name"])

    def test_understand_media_lm_studio_reports_missing_vision_model_clearly(self):
        payload = understand_media(
            {
                "file_id": 12,
                "media_type": "video",
                "file_path": "/Volumes/media/video/clip.mp4",
                "frame_paths": ["/tmp/frame-1.jpg"],
                "context": {"language": "zh-CN", "max_tags": 8, "allow_sensitive_labels": True},
            },
            env={
                "IDEA_WORKER_PROVIDER": "lm_studio",
                "IDEA_WORKER_LM_STUDIO_URL": "http://127.0.0.1:1234",
                "IDEA_WORKER_LM_STUDIO_MODEL": "qwen2.5-vl-7b",
            },
            http_get_json=mock.Mock(return_value={
                "data": [
                    {"id": "huihui-qwen3.5-9b-abliterated"},
                    {"id": "text-embedding-nomic-embed-text-v1.5"},
                ]
            }),
            http_post_json=mock.Mock(side_effect=RuntimeError("model cannot process images")),
        )

        self.assertEqual("placeholder", payload["provider"])
        self.assertEqual("lm_studio", payload["raw_response"]["requested_provider"])
        self.assertIn("model cannot process images", payload["raw_response"]["fallback_reason"])

    def test_understand_media_lm_studio_falls_back_to_first_loaded_chat_model(self):
        def fake_http_post_json(url, body, headers, timeout):
            self.assertEqual("huihui-qwen3.5-9b-abliterated", body["model"])
            return {
                "model": "huihui-qwen3.5-9b-abliterated",
                "choices": [
                    {
                        "message": {
                            "content": json.dumps({
                                "raw_tags": ["图片"],
                                "canonical_candidates": [{"namespace": "content", "name": "图片", "confidence": 0.8}],
                                "summary": "图片画面。",
                                "sensitive_tags": [],
                                "quality_hints": [],
                                "structured_attributes": {"media_type": "image"},
                                "confidence": 0.8,
                            })
                        }
                    }
                ]
            }

        payload = understand_media(
            {
                "file_id": 13,
                "media_type": "image",
                "file_path": "/Volumes/media/photos/poster.jpg",
                "frame_paths": [],
                "context": {"language": "zh-CN", "max_tags": 8, "allow_sensitive_labels": True},
            },
            env={
                "IDEA_WORKER_PROVIDER": "lm_studio",
                "IDEA_WORKER_LM_STUDIO_URL": "http://127.0.0.1:1234",
                "IDEA_WORKER_LM_STUDIO_MODEL": "qwen2.5-vl-7b",
            },
            http_get_json=mock.Mock(return_value={
                "data": [
                    {"id": "huihui-qwen3.5-9b-abliterated"},
                    {"id": "text-embedding-nomic-embed-text-v1.5"},
                ]
            }),
            http_post_json=fake_http_post_json,
        )

        self.assertEqual("lm_studio", payload["provider"])
        self.assertEqual("huihui-qwen3.5-9b-abliterated", payload["model"])
        self.assertEqual("图片画面。", payload["summary"])

    def test_embed_media_returns_pixel_vectors_when_reader_succeeds(self):
        payload = embed_media(
            {
                "embedding_type": "video_frame_visual",
                "media_type": "video",
                "file_path": "/Volumes/media/video/clip.mp4",
                "frames": [
                    {"frame_id": 11, "frame_path": "/tmp/previews/11.jpg", "phash": "0123456789abcdef"},
                    {"frame_id": 12, "frame_path": "/tmp/previews/12.jpg", "phash": "fedcba9876543210"},
                ],
            },
            env={"IDEA_WORKER_EMBED_PROVIDER": "pixel"},
            vector_reader=mock.Mock(side_effect=["[0.1,0.2]", "[0.3,0.4]", "[0.5,0.6]"]),
        )

        self.assertEqual("pixel", payload["provider"])
        self.assertEqual("video_frame_visual", payload["raw_response"]["embedding_type"])
        self.assertEqual(2, len(payload["frame_vectors"]))
        self.assertEqual("[0.3,0.4]", payload["frame_vectors"][0]["vector"])

    def test_embed_media_semantic_uses_understanding_provider_for_image(self):
        def fake_http_post_json(url, body, headers, timeout):
            self.assertEqual("http://127.0.0.1:11434/api/chat", url)
            return {
                "model": "qwen3-vl-8b",
                "message": {
                    "content": """
                    {
                      "raw_tags": ["单人写真", "室内"],
                      "canonical_candidates": [{"namespace":"content","name":"单人写真","confidence":0.93}],
                      "summary": "单人室内写真，画面清晰。",
                      "sensitive_tags": ["写真"],
                      "quality_hints": ["清晰度高"],
                      "structured_attributes": {"media_type":"image","orientation":"portrait"},
                      "confidence": 0.88
                    }
                    """,
                },
            }

        payload = embed_media(
            {
                "media_type": "image",
                "file_path": "/Volumes/media/photos/poster.jpg",
                "image_phash": "0123456789abcdef",
            },
            env={
                "IDEA_WORKER_EMBED_PROVIDER": "semantic",
                "IDEA_WORKER_PROVIDER": "ollama",
                "IDEA_WORKER_OLLAMA_URL": "http://127.0.0.1:11434",
                "IDEA_WORKER_OLLAMA_MODEL": "qwen3-vl-8b",
            },
            http_post_json=fake_http_post_json,
        )

        self.assertEqual("semantic", payload["provider"])
        self.assertEqual("semantic-ollama-qwen3-vl-8b-v1", payload["model"])
        self.assertEqual(64, vector_dims(payload["vector"]))

    def test_build_understanding_prompt_includes_namespace_and_attributes_rules(self):
        prompt = build_understanding_prompt({
            "media_type": "image",
            "file_path": "/Volumes/media/photos/poster.jpg",
            "context": {
                "language": "zh-CN",
                "max_tags": 10,
                "allow_sensitive_labels": True,
            },
        })

        self.assertIn("content, quality, sensitive, person, management", prompt)
        self.assertIn("structured_attributes 尽量包含：media_type, subject_count, capture_type, orientation, has_face, is_sensitive", prompt)
        self.assertIn("allow_sensitive_labels=true", prompt)
        self.assertIn("默认都使用简体中文", prompt)
        self.assertIn("canonical_candidates 必须返回 1-5 个有效候选", prompt)

    def test_embed_media_semantic_builds_frame_vectors_for_video(self):
        def fake_http_post_json(url, body, headers, timeout):
            image_url = body["messages"][0]["images"][0]
            if "frame-11.jpg" in image_url:
                content = """
                {
                  "raw_tags": ["单人", "室内"],
                  "canonical_candidates": [{"namespace":"content","name":"单人","confidence":0.80}],
                  "summary": "单人室内画面。",
                  "sensitive_tags": [],
                  "quality_hints": ["清晰"],
                  "structured_attributes": {"media_type":"image"},
                  "confidence": 0.81
                }
                """
            else:
                content = """
                {
                  "raw_tags": ["双人", "室外"],
                  "canonical_candidates": [{"namespace":"content","name":"双人","confidence":0.82}],
                  "summary": "双人室外画面。",
                  "sensitive_tags": [],
                  "quality_hints": ["清晰"],
                  "structured_attributes": {"media_type":"image"},
                  "confidence": 0.83
                }
                """
            return {
                "model": "qwen3-vl-8b",
                "message": {"content": content},
            }

        with mock.patch("worker.main.encode_file_base64", side_effect=[
            "data:image/jpeg;base64,/tmp/frame-11.jpg",
            "data:image/jpeg;base64,/tmp/frame-12.jpg",
        ]):
            payload = embed_media(
                {
                    "media_type": "video",
                    "frames": [
                        {"frame_id": 11, "frame_path": "/tmp/frame-11.jpg", "phash": "0123456789abcdef"},
                        {"frame_id": 12, "frame_path": "/tmp/frame-12.jpg", "phash": "fedcba9876543210"},
                    ],
                },
                env={
                    "IDEA_WORKER_EMBED_PROVIDER": "semantic",
                    "IDEA_WORKER_PROVIDER": "ollama",
                    "IDEA_WORKER_OLLAMA_URL": "http://127.0.0.1:11434",
                    "IDEA_WORKER_OLLAMA_MODEL": "qwen3-vl-8b",
                },
                http_post_json=fake_http_post_json,
            )

        self.assertEqual("semantic", payload["provider"])
        self.assertEqual(2, len(payload["frame_vectors"]))
        self.assertEqual(64, vector_dims(payload["frame_vectors"][0]["vector"]))
        self.assertNotEqual(payload["frame_vectors"][0]["vector"], payload["frame_vectors"][1]["vector"])

    def test_embed_media_falls_back_when_provider_is_not_supported(self):
        payload = embed_media(
            {
                "media_type": "image",
                "file_path": "/Volumes/media/photos/poster.jpg",
                "image_phash": "0123456789abcdef",
            },
            env={"IDEA_WORKER_EMBED_PROVIDER": "ollama"},
        )

        self.assertEqual("placeholder", payload["provider"])
        self.assertEqual("fallback_placeholder", payload["raw_response"]["mode"])
        self.assertEqual("ollama", payload["raw_response"]["requested_provider"])
        self.assertEqual(64, vector_dims(payload["vector"]))

    def test_embed_media_falls_back_to_placeholder_when_pixel_reader_fails(self):
        payload = embed_media(
            {
                "media_type": "image",
                "file_path": "/Volumes/media/photos/poster.jpg",
                "image_phash": "0123456789abcdef",
            },
            env={"IDEA_WORKER_EMBED_PROVIDER": "pixel"},
            vector_reader=mock.Mock(side_effect=RuntimeError("ffmpeg missing")),
        )

        self.assertEqual("placeholder", payload["provider"])
        self.assertEqual("fallback_placeholder", payload["raw_response"]["mode"])
        self.assertEqual("pixel", payload["raw_response"]["requested_provider"])

    def test_list_models_uses_env_model_overrides(self):
        with mock.patch.dict("os.environ", {
            "IDEA_WORKER_OLLAMA_MODEL": "qwen2.5-vl-7b",
            "IDEA_WORKER_LM_STUDIO_MODEL": "qwen3-vl-8b",
        }, clear=False):
            response = handle_message({
                "request_id": "req-4",
                "type": "list_models",
            })

        self.assertEqual("qwen2.5-vl-7b", response["payload"]["providers"][0]["default_model"])
        self.assertEqual("qwen3-vl-8b", response["payload"]["providers"][1]["default_model"])

    def test_parse_json_object_text_extracts_embedded_json(self):
        payload = parse_json_object_text("说明文字\n{\"ok\": true, \"value\": 2}\n更多说明")

        self.assertEqual({"ok": True, "value": 2}, payload)

    def test_normalize_understanding_result_derives_candidates_when_missing(self):
        payload = normalize_understanding_result(
            {
                "raw_tags": [
                    "/Volumes/media/bad-path.mp4",
                    "JK",
                    "Glasses",
                    "DarkMode",
                ],
                "canonical_candidates": None,
                "summary": "english summary",
                "structured_attributes": {
                    "media_type": "video",
                    "subject_count": "single",
                    "capture_type": "CloseUp",
                    "is_sensitive": True,
                },
                "confidence": 0.85,
            },
            provider="lm_studio",
        )

        self.assertEqual(["JK", "Glasses", "DarkMode"], payload["raw_tags"])
        names = [item["name"] for item in payload["canonical_candidates"]]
        self.assertIn("视频", names)
        self.assertIn("单人", names)
        self.assertIn("特写", names)
        self.assertIn("敏感内容", names)

    def test_build_image_inputs_respects_max_images(self):
        from worker.main import build_image_inputs

        with mock.patch("worker.main.encode_file_base64", side_effect=["img-1", "img-2", "img-3"]) as encode:
            images = build_image_inputs(
                {"media_type": "video", "frame_paths": ["/tmp/a.jpg", "/tmp/b.jpg", "/tmp/c.jpg"]},
                max_images=1,
            )

        self.assertEqual(["img-1"], images)
        self.assertEqual(1, encode.call_count)

    def test_build_image_inputs_passes_resize_option(self):
        from worker.main import build_image_inputs

        with mock.patch("worker.main.encode_file_base64", return_value="img-1") as encode:
            images = build_image_inputs(
                {"media_type": "video", "frame_paths": ["/tmp/a.jpg"]},
                max_images=1,
                resize_max_edge=384,
            )

        self.assertEqual(["img-1"], images)
        encode.assert_called_once_with("/tmp/a.jpg", resize_max_edge=384)

    def test_unknown_message_type_returns_retryable_false(self):
        response = handle_message({
            "request_id": "req-5",
            "type": "something_else",
        })

        self.assertEqual("req-5", response["request_id"])
        self.assertEqual("error", response["type"])
        self.assertFalse(response["ok"])
        self.assertEqual("unsupported_message_type", response["error"]["code"])
        self.assertFalse(response["error"]["retryable"])


if __name__ == "__main__":
    unittest.main()
