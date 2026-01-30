# notif TTS

Subscribe to a notif.sh topic and speak text using ElevenLabs.

## Setup

```bash
pip install notifsh httpx
export NOTIF_API_KEY=nsh_...
export ELEVENLABS_API_KEY=...
```

## Usage

```bash
# Start listening (default topic: tts.speak)
python tts.py

# Custom topic
python tts.py --topic my.notifications

# Custom voice
python tts.py --voice JBFqnCBsd6RMkjVDRZzb
```

Then emit text from anywhere:

```bash
# CLI
notif emit tts.speak '{"text": "Deploy concluído com sucesso"}'

# Python
await notif.emit("tts.speak", {"text": "Build failed on main"})

# TypeScript
await notif.emit("tts.speak", { text: "New user signed up" });

# curl
curl -X POST https://api.notif.sh/api/v1/emit \
  -H "Authorization: Bearer nsh_..." \
  -H "Content-Type: application/json" \
  -d '{"topic": "tts.speak", "data": {"text": "Hello world"}}'
```

## Options

| Flag | Default | Description |
|------|---------|-------------|
| `--topic, -t` | `tts.speak` | Topic to subscribe |
| `--voice, -v` | `JBFqnCBsd6RMkjVDRZzb` | ElevenLabs voice ID |
| `--model, -m` | `eleven_multilingual_v2` | ElevenLabs model |
| `--server, -s` | `https://api.notif.sh` | Notif server |

## Message Format

```json
{"text": "Text to speak"}
```

Only the `text` field is required. Events without it are skipped.
