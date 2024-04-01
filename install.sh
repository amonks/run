#!/bin/sh

# This script downloads the latest release of run from GitHub and installs it 
# to the current directory.

# Print an error message before exiting.
trap 'echo "Error: $0:$LINENO -- installation failed"' ERR 

set -e          # Exit immediately if a command fails
set -o pipefail # Fail a pipeline if any command fails

main() {
    # Check for the existence of a file named "run" in the current directory.
    if [ -f run ]; then
        echo "Error: run is already installed in the current directory."
        echo "To reinstall, remove the existing run binary and execute this script again."
        exit 1
    fi

    OS=$(uname)
    ARCH=$(arch)

    # Determine the download URL for the current operating system and architecture.
    TARGET_ASSET="run_${OS}_${ARCH}.tar.gz"
    DOWNLOAD_URL="https://github.com/amonks/run/releases/latest/download/${TARGET_ASSET}"

    # Attempt the download and extraction. Suppress error output from curl and tar.
    curl -fsSL $DOWNLOAD_URL 2>/dev/null | tar -xz run 2>/dev/null

    echo "Downloaded Run (latest $OS $ARCH) to ./run"
    ./run -version
    echo "Launch it from here or move it to a directory in your PATH."
}

main "$1"
