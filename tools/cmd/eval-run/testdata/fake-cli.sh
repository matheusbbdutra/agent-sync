#!/usr/bin/env bash
# CLI falso para testar o encanamento do routing-runner.sh sem chamar modelo.
# Le e descarta o prompt de stdin e responde sempre o id em FAKE_ANSWER.
set -euo pipefail

if [ -z "${FAKE_ANSWER:-}" ]; then
  echo "fake-cli: defina FAKE_ANSWER com o id de skill a responder" >&2
  exit 2
fi

cat > /dev/null
printf '%s\n' "$FAKE_ANSWER"
