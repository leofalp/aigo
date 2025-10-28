# AIGO

A Go library for working with AI providers and JSON schema generation.

## Requirements

- Go 1.25 or later
- The project uses the experimental `encoding/json/v2` package

## Building and Testing

This project uses Go's experimental `encoding/json/v2` package, which requires the `jsonv2` experiment to be enabled.

### Building

```bash
GOEXPERIMENT=jsonv2 go build ./...
```

### Testing

```bash
GOEXPERIMENT=jsonv2 go test ./...
```

### Running Examples

```bash
GOEXPERIMENT=jsonv2 go run examples/providerExample/main.go
GOEXPERIMENT=jsonv2 go run examples/simpleTool/main.go
```

## Features

- OpenAI API integration
- JSON Schema generation from Go types
- Tool definitions for AI interactions
