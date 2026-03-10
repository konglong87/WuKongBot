---
name: weather
description: Get current weather and forecasts (no API key required).
metadata: {"requires":{"bins":["curl"]}}
---

# Weather

Two free services, no API keys needed. Use `curl` for HTTP requests.

## Open-Meteo (Primary)
Free, no key, good for programmatic use:
```bash
curl -s "https://api.open-meteo.com/v1/forecast?latitude=51.5&longitude=-0.12&current_weather=true"
```
Returns JSON with temperature, windspeed, and weathercode.

Find coordinates for a city, then query.

**Docs:** https://open-meteo.com/en/docs


## wttr.in (Fallback)

Quick one-liner for current weather:
```bash
curl -s "wttr.in/London?format=3"
# Output: London: +8C
```

Compact format:
```bash
curl -s "wttr.in/London?format=%l:+%c+%t+%h+%w"
# Output: London: +8C 71% 5km/h
```

Full forecast:
```bash
curl -s "wttr.in/London?T"
```

## Format Codes

| Code | Description |
|------|-------------|
| `%c` | Weather condition |
| `%t` | Temperature |
| `%h` | Humidity |
| `%w` | Wind |
| `%l` | Location |
| `%m` | Moon phase |

## Tips

- URL-encode spaces: `wttr.in/New+York`
- Airport codes: `wttr.in/JFK`
- Units: `?m` (metric) `?u` (USCS)
- Today only: `?1` | Current only: `?0`
- PNG image: `curl -s "wttr.in/Berlin.png" -o /tmp/weather.png`