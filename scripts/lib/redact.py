#!/usr/bin/env python3
"""Redact diagnostic streams before they reach a log or terminal."""
import os
import json
import re
import sys

values = set()
for key, value in os.environ.items():
    if re.search(r'(TOKEN|SECRET|PASSWORD|PASSWD|CREDENTIAL|PRIVATE_KEY|API_KEY|TASK_CONTEXT)', key, re.I):
        values.update(part for part in value.splitlines() if len(part) >= 4)
values = sorted(values, key=len, reverse=True)
secret_key = re.compile(r'(^|[_-])(token|secret|password|passwd|credential|private[_-]?key|api[_-]?key|authorization|cookie)($|[_-])|(?:access|refresh|auth|api)(?:Token|Key)$', re.I)

def redact_text(text):
    for value in values:
        text = text.replace(value, '[REDACTED]')
        text = text.replace(json.dumps(value, ensure_ascii=False)[1:-1], '[REDACTED]')
    text = re.sub(r'(https?://)[^/\s@]+@', r'\1[REDACTED]@', text)
    return re.sub(r'(?i)(bearer|basic)\s+[A-Za-z0-9+/=_\-.]+', r'\1 [REDACTED]', text)

def redact_json(value):
    if isinstance(value, dict):
        return {key: '[REDACTED]' if secret_key.search(key) else redact_json(item) for key, item in value.items()}
    if isinstance(value, list):
        return [redact_json(item) for item in value]
    if isinstance(value, str):
        if 'PRIVATE KEY-----' in value:
            return '[REDACTED PRIVATE KEY]'
        return redact_text(value)
    return value

private_key = False
for line in sys.stdin:
    try:
        parsed = json.loads(line)
    except ValueError:
        pass
    else:
        sys.stdout.write(json.dumps(redact_json(parsed), ensure_ascii=False) + '\n')
        continue
    # Pretty JSON often supplies a complete property on each physical line.
    # Preserve its delimiters while masking decoded strings (including PEMs).
    property_line = re.match(r'^(\s*)("(?:\\.|[^"\\])*")(\s*:\s*)(.*?)(,?)(\s*)$', line)
    if property_line:
        try:
            key = json.loads(property_line[2])
            value = json.loads(property_line[4])
        except ValueError:
            pass
        else:
            value = '[REDACTED]' if secret_key.search(key) else redact_json(value)
            sys.stdout.write(property_line[1] + property_line[2] + property_line[3] + json.dumps(value, ensure_ascii=False) + property_line[5] + property_line[6])
            continue
    if '-----BEGIN ' in line and 'PRIVATE KEY-----' in line:
        private_key = True
    if private_key:
        sys.stdout.write('[REDACTED PRIVATE KEY]\n')
        if '-----END ' in line and 'PRIVATE KEY-----' in line:
            private_key = False
        continue
    line = redact_text(line)
    line = re.sub(r'''(?ix)((?:["']?(?:[\w.-]*(?:token|secret|password|api[_-]?key)|authorization|cookie)["']?)\s*[:=]\s*)("(?:\\.|[^"\\\n])*"|'[^'\n]*'|[^\s&,;}\]]+)''', r'\1"[REDACTED]"', line)
    sys.stdout.write(line)
