# Scroll

## Overview

Scroll (`cmd/scroll`) is a standalone tailing log pager that reuses the `logview` component. It continuously reads from stdin or a file and provides interactive search and navigation.

## Usage

```
scroll [file]
scroll -       # read from stdin
scroll         # read from stdin (default)
```

## Behavior

- Reads input continuously (tailing), feeding content to a `logview.Model`.
- Interactive features: paging, scrolling, incremental regex search.
- Alt screen mode with mouse support.

## Performance Characteristics

- Loads the whole file into memory before starting.
- Perceptible latency: search query changes in files larger than ~50 MB; opening files larger than ~500 MB.
- No noticeable latency for scrolling or tailing, even in very large files (tested up to 15 GB).

## Architecture

- `Sink` is a function type that receives strings and sends them as `writeMsg` to the BubbleTea program.
- `tailStdin()` and `tailFile(filename)` continuously read and pipe to the sink.
- The `scroll` model wraps `logview.Model` and forwards all messages to it.
