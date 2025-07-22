# Raspberry Pi Client

Turn a Raspberry pi into a dedicated Slinky streaming device.

**Note**: These instructions will likely work for any Debian-based system and, with modifications, for any other OS too.

## Requirements

1. Slinky installed and running
2. A [Raspberry Pi 4 or newer](https://www.raspberrypi.com/products/raspberry-pi-5/)
   - Raspberry Pi 5 at the time of writing was cheaper and more powerful
   - Ensure your Pi device supports hardware H.265 decoding
   - RAM shouldn't matter, but has been tested with 4GB
3. An IR receiver for the Raspberry Pi
   - A [Cable Matters Infrared Remote Extender Cable](https://www.amazon.co.uk/gp/product/B0CNLF163B) was tested for this project
4. A Micro SD card for the Raspberry Pi
   - It was tested with a 64 GiB Class 10 card
5. (Optional) a case for the Raspberry Pi

## Setup

You should install Raspberry Pi OS Lite on your Raspberry Pi. See the [official site](https://www.raspberrypi.com/software/operating-systems/) for download and installation instructions. This was tested with the 64-bit release.

### Install Docker

The client is packaged in Docker for portability and installation simplicity. Follow the [official instructions](https://docs.docker.com/engine/install/debian/) to install Docker Engine on Debian (or follow the [Raspberry Pi instructions](https://docs.docker.com/engine/install/raspberry-pi-os/) if using the 32-bit OS).

### Configure Slinky Client

1. Install required system-wide packages:

   ```bash
   sudo apt update && sudo apt install -y --no-install-recommends \
    git \
    xserver-xorg \
    x11-xserver-utils \
    xinit \
    openbox
   ```

2. Clone this repo:

   ```bash
   git clone https://github.com/mattsday/slinky
   ```

3. Create a Slinky group and user (if you pick another UID, edit your `docker-compose.yaml` file with it - likewise, change the home directory as needed):

   ```bash
   sudo groupadd --gid 850 slinky
   sudo useradd -c "Slinky" -M -u 850 -g slinky -d /etc/slinky -s /sbin/nologin slinky
   ```

4. Create a Slinky configuration file:

   ```bash
   cp slinky/pi/client/config.sh.example slinky/pi/client/config.sh
   ```

5. Edit the config file - in particular set your SLINKY_ENDPOINT to your slinky server (e.g. `slinky.myserver.com`)

6. Copy your `config.sh` to `/etc/slinky/config.sh` or edit your `docker-compose.yaml` file and select a location for it

7. Give your slinky user access to the config file:

   ```bash
   sudo chown slinky:slinky /etc/slinky/config.sh
   ```

8. Build the client image:

   ```bash
   cd slinky/pi/client
   docker compose build
   ```

9. Run the client as a daemon:

   ```bash
   docker compose up -d
   ```

This should startup a Slinky client
