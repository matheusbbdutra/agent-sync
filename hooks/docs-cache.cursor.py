#!/usr/bin/env python3
"""Cache passivo de docs no Cursor (WebFetch postToolUse ou context7 afterMCPExecution)."""
import json
import subprocess
import sys

writer = sys.argv[1]
mode = sys.argv[2] if len(sys.argv) > 2 else "auto"

try:
    data = json.load(sys.stdin)
except Exception:
    print("{}")
    sys.exit(0)


def as_text(value):
    if isinstance(value, str):
        # tool_output / result_json geralmente vem JSON-stringified
        try:
            parsed = json.loads(value)
            return as_text(parsed)
        except Exception:
            return value
    if isinstance(value, dict):
        for key in ("result", "content", "text", "output", "markdown", "stdout"):
            v = value.get(key)
            if isinstance(v, str) and v.strip():
                return v
            if isinstance(v, list):
                parts = []
                for item in v:
                    if isinstance(item, dict) and isinstance(item.get("text"), str):
                        parts.append(item["text"])
                    elif isinstance(item, str):
                        parts.append(item)
                if parts:
                    return "\n".join(parts)
        return json.dumps(value, ensure_ascii=False)
    if isinstance(value, list):
        return "\n".join(as_text(v) for v in value if v)
    return ""


def parse_maybe_json(value):
    if isinstance(value, dict):
        return value
    if isinstance(value, str):
        try:
            parsed = json.loads(value)
            if isinstance(parsed, dict):
                return parsed
        except Exception:
            return {}
    return {}


url = ""
text = ""
tool_name = data.get("tool_name") or ""
tool_input = data.get("tool_input")

if mode in ("webfetch", "auto") and tool_name == "WebFetch":
    inp = parse_maybe_json(tool_input) if not isinstance(tool_input, dict) else tool_input
    url = inp.get("url") or ""
    text = as_text(data.get("tool_output") or data.get("tool_response"))
elif mode in ("mcp", "auto"):
    server = (data.get("mcp_server_name") or "").lower()
    if "context7" in server or tool_name.endswith("query-docs") or tool_name == "query-docs":
        inp = parse_maybe_json(tool_input) if not isinstance(tool_input, dict) else (tool_input or {})
        lib = inp.get("libraryId") or inp.get("context7CompatibleLibraryID") or "unknown-library"
        query = inp.get("query") or inp.get("topic") or "index"
        url = f"context7:/{lib}/{query}"
        text = as_text(data.get("result_json") or data.get("tool_output") or data.get("tool_response"))

if url and text:
    payload = json.dumps({"url": url, "contentType": "text/plain", "text": text})
    try:
        subprocess.run([writer], input=payload, text=True, capture_output=True, timeout=10)
    except Exception:
        pass

print("{}")
