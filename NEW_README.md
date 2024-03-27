# 🏃🏽‍♀️🏃🏾‍♂️🏃🏻‍♀️💨 **_⸻ RUN_**

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/amonks/run?logo=go&logoColor=white&label=%20&labelColor=gray&color=00ADD8)
[![Godoc](https://img.shields.io/badge/go-docs-blue?logo=go&logoColor=white&label=%20&labelColor=gray&color=blue)](https://amonks.github.io/run)
[![Go Report Card](https://goreportcard.com/badge/github.com/amonks/run)](https://goreportcard.com/report/github.com/amonks/run)

RUN is a task runner that simplifies executing and managing tasks defined in `tasks.toml` files. It provides a versatile set of features making it well suited for a range of use cases, from simple build scripts to complex development workflows.

<img alt="interactive TUI" src="screenshots/tui.gif?raw=true" />

## Features

- **Flexible Task Configuration**: Support for complex task dependencies, environment variable injection, and file watching for automatic task re-execution.
- **Interactive TUI**: Full mouse support for managing long-lived tasks.
- **Non-Interactive Output**: Interleaved output suitable for short-lived processes.
- **Go API**: Extensibility through a Go programming interface.

## Installation

RUN can be installed as a single binary or via the Go command line tool:

### Pre-compiled binary

Download the latest release from the [releases page](https://github.com/amonks/run/releases), and extract the binary to a directory in your `PATH`. This can be done in a single command like:

    $ curl -sL https://github.com/amonks/run/releases/download/<RELEASE_VERSION>/run_<RELEASE_ARCH>.tar.gz | tar -x && chmod +x run && mv run ~/go/bin

### Using Go

If you already use go and have it installed, you can install RUN with the go command line tool.

    $ go install github.com/amonks/run/cmd/run@latest

## Quick Start

Follow these steps to get started with RUN:

1. **Create a `tasks.toml` file**
    
        $ touch tasks.toml

2. **Define a task in that `tasks.toml`**
    ```toml
    [[task]]
      id = "hello"
      type = "short"
      cmd = "echo Hello, World!"
    ```

3. **Run the task and see task output**
      
    <img alt="hello-gif" src="screenshots/hello.gif">