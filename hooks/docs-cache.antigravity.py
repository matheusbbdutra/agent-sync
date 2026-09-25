import json
import subprocess
import sys

writer = sys.argv[1]

try:
    data = json.load(sys.stdin)
except Exception:
    print("{}")
    sys.exit(0)

tool_call = data.get("toolCall") or {}
name = tool_call.get("name", "")
args = tool_call.get("args") or {}
step_idx = data.get("stepIdx")
transcript_path = data.get("transcriptPath")

url = ""

if name == "read_url_content":
    # Campo exato nao confirmado na documentacao publica; tenta as variantes
    # mais prováveis dado o padrao PascalCase usado pelos outros tools do
    # Antigravity (CommandLine, AbsolutePath, ServerName...).
    url = args.get("Url") or args.get("URL") or args.get("url") or ""
elif name == "call_mcp_tool" and args.get("ServerName") == "context7" and args.get("ToolName") == "query-docs":
    inner = args.get("Arguments") or {}
    lib = inner.get("libraryId", "unknown-library")
    query = inner.get("query", "index")
    url = f"context7:/{lib}/{query}"

if not url or step_idx is None or not transcript_path:
    print("{}")
    sys.exit(0)


def find_result_text(path, after_step):
    # O resultado da tool aparece como a entrada GENERIC seguinte no
    # transcript (step_index == after_step + 1), campo "content".
    target = after_step + 1
    try:
        with open(path, "r", encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                try:
                    entry = json.loads(line)
                except Exception:
                    continue
                if entry.get("step_index") == target and entry.get("type") == "GENERIC":
                    content = entry.get("content")
                    if isinstance(content, str):
                        return content
    except OSError:
        return ""
    return ""


text = find_result_text(transcript_path, step_idx)

if url and text:
    payload = json.dumps({"url": url, "contentType": "text/plain", "text": text})
    try:
        subprocess.run([writer], input=payload, text=True, capture_output=True, timeout=10)
    except Exception:
        pass

print("{}")
