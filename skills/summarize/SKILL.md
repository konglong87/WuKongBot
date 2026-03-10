---
name: summarize
description: Summarize or extract text/transcripts from URLs, podcasts, and local files. Great for "transcribe this YouTube/video".
metadata: {"requires":{"bins":["summarize"]}}
---

# Summarize

Fast CLI to summarize URLs, local files, and YouTube links.

## When to Use (Trigger Phrases)

Use this skill when the user asks:
- "use summarize.sh"
- "what's this link/video about?"
- "summarize this URL/article"
- "transcribe this YouTube/video"

## Quick Start

```bash
summarize "https://example.com" --model google/gemini-3-flash-preview
summarize "/path/to/file.pdf" --model google/gemini-3-flash-preview
summarize "https://youtu.be/dQw4w9WgXcQ" --youtube auto
```

## YouTube: Summary vs Transcript

Best-effort transcript (URLs only):
```bash
summarize "https://youtu.be/dQw4w9WgXcQ" --youtube auto --extract-only
```

If the user asked for a transcript but it's huge, return a tight summary first, then ask which section/time range to expand.

## Model and API Keys

Set the API key for your chosen provider:
- OpenAI: `OPENAI_API_KEY`
- Anthropic: `ANTHROPIC_API_KEY`
- xAI: `XAI_API_KEY`
- Google: `GEMINI_API_KEY`

Default model is `google/gemini-3-flash-preview` if none is set.

## Useful Flags

| Flag | Description |
|------|-------------|
| `--length short\|medium\|long\|xl\|xxl\|<chars>` | Summary length |
| `--max-output-tokens <count>` | Token limit |
| `--extract-only` | Extract only (URLs) |
| `--json` | Machine readable output |
| `--firecrawl auto\|off\|always` | Fallback extraction |
| `--youtube auto` | Apify fallback if `APIFY_API_TOKEN` set |

## Config File

Optional: `~/.summarize/config.json`

```json
{ "model": "openai/gpt-4" }
```

## Optional Services

- `FIRECRAWL_API_KEY` for blocked sites
- `APIFY_API_TOKEN` for YouTube fallback
