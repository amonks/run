#!/bin/bash

# By default, this script will install the latest version of run to $HOME/.local/bin.
install_dir="$HOME/.local/bin"
version="latest"
version_str="v1.0.0-beta.24"

# Detect OS and Architecture
os=""
arch=$(uname -m)
os_type=$(uname -s)

case "$os_type" in
    Linux*)  os="Linux";;
    Darwin*) os="Darwin";;
    *) echo "Unsupported OS" && exit 1;;
esac

case "$arch" in
    x86_64*) arch="x86_64";;
    arm64*)  arch="arm64";;
    i386*)
        if [ "$os" = "Linux" ]; then
            arch="i386"
        else
            echo "The i386 architecture is only supported on Linux."
            exit 1
        fi
        ;;
    *) echo "Unsupported architecture" && exit 1 ;;
esac

# Construct the download URL
base_url="https://github.com/amonks/run/releases/download/${version_str}"
filename="run_${os}_${arch}.tar.gz"
url="${base_url}/${filename}"

# Download the tarball
temp_dir=$(mktemp -d)
curl -LsS $url -o $temp_dir/run.tar.gz || {
    echo "Failed to download $url"
    exit 1
}

# Extract the tarball to the installation directory
mkdir -p $install_dir
tar -xzf $temp_dir/run.tar.gz -C $install_dir
chmod u+x $install_dir/run

# Clean up the temporary directory
rm -rf $temp_dir

echo "run has been installed to $install_dir"

# Check if the installation directory is in the PATH
if [[ ":$PATH:" != *":$install_dir:"* ]]; then
    echo "WARNING: $install_dir is not in your PATH"
    echo "Consider adding $install_dir to your PATH"
else
    echo "$install_dir is already in your PATH"
fi

echo "Installation complete!"