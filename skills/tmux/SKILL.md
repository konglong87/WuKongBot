---
name: tmux
description: Remote-control tmux sessions for interactive CLIs by sending keystrokes and scraping pane output.
metadata: {"os":["darwin","linux"],"requires":{"bins":["tmux"]}}
---

# tmux Skill

Use tmux only when you need an interactive TTY. Prefer exec background mode for long-running, non-interactive tasks.

## Quickstart (Isolated Socket, Exec Tool)

```bash
SOCKET_DIR="${wukongbot_TMUX_SOCKET_DIR:-${TMPDIR:-/tmp}/wukongbot-tmux-sockets}"
mkdir -p "$SOCKET_DIR"
SOCKET="$SOCKET_DIR/wukongbot.sock"
SESSION=wukongbot

tmux -S "$SOCKET" new -d -s "$SESSION" -n shell
tmux -S "$SOCKET" send-keys -t "$SESSION":0.0 -- 'python3 -q' Enter
tmux -S "$SOCKET" capture-pane -p -J -t "$SESSION":0.0 -S -200
```

After starting a session, always print monitor commands:

```
To monitor:
  tmux -S "$SOCKET" attach -t "$SESSION"
  tmux -S "$SOCKET" capture-pane -p -J -t "$SESSION":0.0 -S -200
```

## Socket Convention

- Use `wukongbot_TMUX_SOCKET_DIR` environment variable
- Default socket path: `"$wukongbot_TMUX_SOCKET_DIR/wukongbot.sock"`

## Targeting Panes and Naming

- Target format: `session:window.pane` (defaults to `:0.0`)
- Keep names short; avoid spaces
- Inspect: `tmux -S "$SOCKET" list-sessions`, `tmux -S "$SOCKET" list-panes -a`

## Sending Input Safely

- Prefer literal sends: `tmux -S "$SOCKET" send-keys -t target -l -- "$cmd"`
- Control keys: `tmux -S "$SOCKET" send-keys -t target C-c`

## Watching Output

- Capture recent history: `tmux -S "$SOCKET" capture-pane -p -J -t target -S -200`
- Wait for prompts: use a polling loop to check for prompt patterns
- Attaching is OK; detach with `Ctrl+b d`

## Spawning Processes

For python REPLs, set `PYTHON_BASIC_REPL=1` (non-basic REPL breaks send-keys flows).

## Windows / WSL

- tmux is supported on macOS/Linux
- On Windows, use WSL and install tmux inside WSL
- This skill is gated to `darwin`/`linux` and requires `tmux` on PATH

## Cleanup

- Kill a session: `tmux -S "$SOCKET" kill-session -t "$SESSION"`
- Kill all sessions on a socket:
  ```bash
  tmux -S "$SOCKET" list-sessions -F '#{session_name}' | xargs -r -n1 tmux -S "$SOCKET" kill-session -t
  ```
- Remove everything on the private socket: `tmux -S "$SOCKET" kill-server`
