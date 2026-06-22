FLASH(1)

# NAME

flash - spaced-repetition flashcard CLI

# SYNOPSIS

*flash* [flags] _deck.md_++
*flash* [flags] _host:deck.md_++
*flash* push _host:deck.md_++
*flash* serve

# DESCRIPTION

*flash* is a terminal flashcard program that uses spaced repetition (modified
FSRS) to schedule reviews.  Cards are stored in a plain Markdown file;
progress is persisted in a SQLite database.

A *serve* subcommand exposes a REST API so card state lives on a shared
server.  Remote clients connect via the *host:deck.md* syntax.

# COMMANDS

*flash* _deck.md_
	Study a local deck.  Parses _deck.md_, opens (or creates) _deck_.db,
	and launches the interactive TUI.

*flash* _host:deck.md_
	Study a remote deck.  Card state is managed by the server; evaluation
	(keyword matching + LLM) runs locally.

*flash push* _host:deck.md_
	Upload the local file _deck.md_ to the server and sync its card list.

*flash serve*
	Start the HTTP API server.  Requires *serve_token* to be set.

# FLAGS

*-db* _path_
	Override the SQLite database path (default: _deck_.db next to the deck).

*-reset*
	Reset all card progress for the deck, then exit.

# DECK FORMAT

A deck is a UTF-8 Markdown file.  Cards are separated by blank lines.
The first line of each card is the *concept*; remaining lines are the
*reference* (supports Markdown bold for keywords).

```
What is spaced repetition?
A learning technique that schedules reviews at **increasing intervals**.

What is FSRS?
A modern spaced-repetition algorithm based on **memory stability** and
**difficulty**.
```

# CONFIGURATION

Settings are resolved in this order (later overrides earlier):

. Built-in defaults
. *flash.cfg* next to the deck file, then *./flash.cfg* (TOML)
. Environment variables
. Command-line flags

## Config file (flash.cfg)

```
ollama_url   = "http://localhost:11434/api/chat"
ollama_user  = ""
ollama_pass  = ""
ollama_model = "qwen3:4b"
threshold    = 0.7

# db = "deck.db"

# --- server (flash serve) ---
# serve_host  = "0.0.0.0"
# serve_port  = 8765
# serve_token = "change-me"
# serve_data  = "."

# --- client remote (flash host:deck.md) ---
# remote_port  = 8765
# remote_token = "change-me"
```

## Environment variables

[[ *Variable*
:[ *Config key*
|  FLASH_OLLAMA_URL
:  ollama_url
|  FLASH_OLLAMA_USER
:  ollama_user
|  FLASH_OLLAMA_PASS
:  ollama_pass
|  FLASH_OLLAMA_MODEL
:  ollama_model
|  FLASH_DB
:  db
|  FLASH_SERVE_HOST
:  serve_host
|  FLASH_SERVE_PORT
:  serve_port
|  FLASH_SERVE_TOKEN
:  serve_token
|  FLASH_SERVE_DATA
:  serve_data
|  FLASH_REMOTE_PORT
:  remote_port
|  FLASH_REMOTE_TOKEN
:  remote_token

# TUI KEYS

[[ *Key*
:[ *Action*
|  _Enter_
:  Submit answer / advance
|  _Ctrl+C_, _q_
:  Quit

# SEE ALSO

*scdoc*(1)

# AUTHORS

Paul Chambaz <paul.chambaz@tutanota.com>
