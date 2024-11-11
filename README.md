# JavaDoc Parser

A tool to generate reports on deprecated elements in the PaperMC API across different Minecraft versions.

![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/thelooter/paper-deprecations/pages.yaml?label=Build%20%26%20Deploy)

## Description

This tool parses the PaperMC JavaDocs to create a comprehensive report of all deprecated elements, organized by Minecraft version. It helps plugin developers track API changes and plan updates accordingly.

## Features

- Lists all deprecated classes, methods, and fields
- Groups deprecations by Minecraft version
- Links directly to JavaDoc sources
- Caches results to avoid unnecessary fetches
- Generates static HTML report

## Building & Development

### Prerequisites

- Go 1.23 or higher
- [templ](https://github.com/a-h/templ) for HTML templating

### Setup Development Environment

1. Clone the repository
```bash
git clone https://github.com/thelooter/JavaDocParser.git
cd JavaDocParser
```

2. Install dependencies

```bash
go mod download
```

3. Install `templ` (required for HTML generation)
```bash
go install github.com/a-h/templ/cmd/templ@latest
```

4. Generate template code

```bash
templ generate
```

5. Build the project

```bash
go build -o JavaDocParser -v ./...
```

### Available flags:

- `-c, --cache`: Use cached data instead of fetching new data
- `-o, --output-dir`: Directory to store generated files (default: ".")

### Generated Files

- `deprecations.json`: Cache file containing parsed deprecation data
- `deprecations.html`: Generated HTML report

## Limitations
- Does not track methods already removed in previous versions
- Some deprecated elements may not show version information if not specified in API docs

## License

[MIT License](https://choosealicense.com/licenses/mit/)