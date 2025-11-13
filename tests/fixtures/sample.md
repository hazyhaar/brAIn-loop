# Brainloop Documentation

Brainloop is a Cerebras-powered MCP server for code generation and intelligent reading.

## Features

- **Code Generation**: Generate Go, Python, SQL, and other code types
- **Intelligent Reading**: Analyze SQLite databases, markdown files, code, and configs
- **Pattern Extraction**: Automatically detect project patterns
- **Progressive Disclosure**: Single MCP tool with 11 actions

## Quick Start

### Installation

```bash
cd /workspace/projets/brainloop
go mod download
mage build
```

### Configuration

Set your Cerebras API key:

```sql
UPDATE secrets SET secret_value='sk-your-key' WHERE secret_name='CEREBRAS_API_KEY';
```

## Usage

### Generate File

```json
{
  "action": "generate_file",
  "params": {
    "verified_prompt": "Create a user struct with validation",
    "output_path": "user.go",
    "code_type": "go"
  }
}
```

### Read SQLite

```json
{
  "action": "read_sqlite",
  "params": {
    "db_path": "/path/to/database.db",
    "max_sample_rows": 5
  }
}
```

## Architecture

Brainloop follows the HOROS 4-BDD pattern:

1. **input.db**: External sources
2. **lifecycle.db**: Operational state
3. **output.db**: Published results
4. **metadata.db**: Secrets and telemetry

## Links

- [HOROS Documentation](https://example.com/horos)
- [Cerebras API](https://cerebras.ai)

![Architecture Diagram](architecture.png)
