#!/bin/bash

# Removarr Installation Script for Seedboxes
# Based on Unpackerr installation script

set -e

# Get the latest release tag from GitHub API
TAG=$(curl -s https://api.github.com/repos/YOUR_USERNAME/removarr/releases/latest | \
  grep "tag_name" | cut -d '"' -f 4)

# Check system architecture and assign it to ARCH variable
ARCH="$(uname -m)"
if [ "$ARCH" = "x86_64" ]; then
  ARCH="amd64"
elif [ "$ARCH" = "aarch64" ]; then
  ARCH="arm64"
elif [ "$ARCH" = "armv7l" ]; then
  ARCH="arm"
fi

# Construct the download URL
URL="https://github.com/YOUR_USERNAME/removarr/releases/download/$TAG/removarr.${ARCH}.linux.gz"

# Download and extract the binary
mkdir -p $HOME/removarr
echo "Downloading Removarr..."
wget $URL -O - | gunzip > $HOME/removarr/removarr
chmod 0755 $HOME/removarr/removarr

# Download the example config file
echo "Downloading example configuration..."
wget https://raw.githubusercontent.com/YOUR_USERNAME/removarr/$TAG/config.example.yaml \
  -O $HOME/removarr/config.example.yaml
chmod 0600 $HOME/removarr/config.example.yaml

# Copy to config.yaml if it doesn't exist
if [ ! -f "$HOME/removarr/config.yaml" ]; then
  cp $HOME/removarr/config.example.yaml $HOME/removarr/config.yaml
  echo "Created config.yaml from example. Please edit it before running."
fi

# Open nano to edit config
echo ""
echo "Opening configuration file for editing..."
echo "Please configure your database connection and integration settings."
echo "Press Ctrl+X to save and exit when done."
sleep 2
nano $HOME/removarr/config.yaml

# Set up a screen session to run removarr in the background
echo ""
echo "Starting Removarr in a screen session..."
screen -dmS removarr $HOME/removarr/removarr -config $HOME/removarr/config.yaml

# Add a cron job to start the screen session on reboot
echo ""
echo "Adding cron job for auto-start on reboot..."
(crontab -l 2>/dev/null; \
  echo "@reboot screen -dmS removarr $HOME/removarr/removarr -config $HOME/removarr/config.yaml") | \
  crontab -

echo ""
echo "Installation complete!"
echo ""
echo "Removarr is now running in a screen session."
echo "To attach to the session: screen -r removarr"
echo "To detach: Press Ctrl+A then D"
echo "To stop: Attach and press Ctrl+C"
echo ""
echo "Configuration file: $HOME/removarr/config.yaml"
echo "Logs: Check the screen session or configure logging in config.yaml"

