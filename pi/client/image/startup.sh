#!/bin/sh

echo "Slinky Client Starting..."

# --- Load Configuration ---
# Check if the config file exists and source it to load environment variables
if [ -f /etc/slinky/slinky.sh ]; then
  echo "Loading configuration from /etc/slinky/slinky.sh"
  . /etc/slinky/slinky.sh
else
  echo "Warning: Configuration file not found. Using default endpoint."
  # Set a default endpoint if the config file is missing
  export SLINKY_ENDPOINT="http://192.168.1.100:8080/stream/0.ts"
fi

# --- Start LIRC service ---
echo "Starting LIRC daemon..."
lircd --nodaemon &
sleep 2

# --- Launch VLC ---
# Use the SLINKY_ENDPOINT variable from your config file
echo "Launching VLC with endpoint: $SLINKY_ENDPOINT"
vlc --fullscreen "$SLINKY_ENDPOINT" &

# --- Start Remote Listener (Placeholder) ---
echo "Remote control listener would start here."

# Keep the container running
wait
