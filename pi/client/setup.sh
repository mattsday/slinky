#!/bin/sh
# This setup file configures the re-requisites from the README.md file. It makes some assumptions:
# 1. You are running 64-bit Raspberry Pi OS
# 2. You have set things up exactly like the README.md file and wish to have things setup that way too

install_docker() {
  sudo apt-get update
  sudo apt-get install -y ca-certificates curl
  sudo install -m 0755 -d /etc/apt/keyrings
  sudo curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
  sudo chmod a+r /etc/apt/keyrings/docker.asc
  echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" |
    sudo tee /etc/apt/sources.list.d/docker.list >/dev/null
  sudo apt-get update
  sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
}

setup_users() {
  if ! getent group slinky >/dev/null 2>&1; then
    sudo groupadd --gid 850 slinky
  fi
  if ! id -u slinky >/dev/null 2>&1; then
    sudo useradd -c "Slinky" -M -u 850 -g slinky -d /etc/slinky -s /sbin/nologin slinky
  fi
}

setup_config() {
  cp config.sh.example config.sh
  sed -i 's/SLINKY_ENDPOINT/'"${SLINKY_ENDPOINT}"'/g' config.sh
  sudo mkdir -p /etc/slinky
  sudo ln -s "${PWD}/config.sh" /etc/slinky/config.sh
  sudo chown -R slinky:slinky /etc/slinky
}

build_image() {
  cd client || exit 1
  docker compose build
  cd .. || exit 1
}

run_image() {
  cd client || exit 1
  docker compose up -d
  cd .. || exit 1
}

main() {
  set -e
  install_docker
  setup_users
  setup_config
  build_image
  run_image
}

main
