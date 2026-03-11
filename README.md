# WaniTani

A terminal-based WaniKani client. Learn kanji, radicals, and vocabulary right from your terminal.

## Install

```
go install github.com/hhofner/wanitani@latest
```

## Usage

```
wanitani
```

Use `/add-token` to set your WaniKani API key, then `/learn` to start.

## Features

- Lesson mode with composition, meaning, reading, and context tabs
- Quiz after each lesson batch with answer checking
- Romaji-to-hiragana live conversion for reading answers
- Incorrect answers are re-queued until answered correctly
- Completed lessons are submitted to WaniKani automatically
- Lesson/review count dashboard
- Token saved locally (`~/.wanitani/token`)
- Tab-cycling command suggestions

## Coming soon

- Review mode
- SRS answer submission
- Lesson batch size setting
- Session statistics

## Commands

| Command | Description |
|---|---|
| `/help` | Show available commands |
| `/add-token` | Set your WaniKani API token |
| `/learn` | Start a lesson session |
| `/review` | Start a review session |
| `/quit` | Exit |

## License

MIT
