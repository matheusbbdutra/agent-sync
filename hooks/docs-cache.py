import json
import subprocess
import sys

writer = sys.argv[1]

try:
    data = json.load(sys.stdin)
except Exception:
    print("{}")
    sys.exit(0)

tool_name = data.get("tool_name", "")
tool_input = data.get("tool_input") or {}
tool_response = data.get("tool_response")


def as_text(value):
    if isinstance(value, str):
        return value
    if isinstance(value, dict):
        for key in ("result", "content", "text", "output"):
            v = value.get(key)
            if isinstance(v, str):
                return v
        return json.dumps(value, ensure_ascii=False)
    return ""


url = ""
text = ""

if tool_name == "WebFetch":
    url = tool_input.get("url", "")
    text = as_text(tool_response)
elif tool_name.startswith("mcp__context7__query-docs") or tool_name.endswith("query-docs"):
    lib = tool_input.get("libraryId") or tool_input.get("context7CompatibleLibraryID") or "unknown-library"
    query = tool_input.get("query") or tool_input.get("topic") or "index"
    url = f"context7:/{lib}/{query}"
    text = as_text(tool_response)

if url and text:
    payload = json.dumps({"url": url, "contentType": "text/plain", "text": text})
    try:
        subprocess.run([writer], input=payload, text=True, capture_output=True, timeout=10)
    except Exception:
        pass

print("{}")
