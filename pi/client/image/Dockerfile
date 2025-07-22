# Start from a lightweight, official Raspberry Pi OS image
FROM debian:bookworm-slim

# Set the frontend to noninteractive to prevent installation prompts
ENV DEBIAN_FRONTEND=noninteractive

# Create a dedicated group and user for the application
# Using specific IDs as requested for consistent permissions.
RUN groupadd --gid 850 slinky &&
  useradd --uid 850 --gid 850 --create-home slinky

# Install VLC, the LIRC remote control software, and procps (for process management)
RUN apt-get update &&
  apt-get install -y --no-install-recommends \
    vlc \
    lirc \
    procps &&
  rm -rf /var/lib/apt/lists/*

# Switch to the newly created user's home directory
WORKDIR /home/slinky/app

# Copy your custom startup script into the container's app directory
COPY startup.sh .

# Make the startup script executable
RUN chmod +x startup.sh

# Set the default command to run when the container starts
CMD [ "./startup.sh" ]
