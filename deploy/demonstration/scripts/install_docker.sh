#!/bin/bash
set -euo pipefail

# -----------------------------------------------------------------------------
# Docker & Docker Compose installation script
# -----------------------------------------------------------------------------
# This script installs Docker Engine and Docker Compose on an EC2 instance.
# Steps:
#   1. Update system packages
#   2. Install Docker
#   3. Enable and start Docker service
#   4. Add ec2-user to the docker group
#   5. Download and install Docker Compose
#   6. Verify installation
# -----------------------------------------------------------------------------

# Log file for troubleshooting
LOG_FILE="/var/log/docker-install.log"
exec > >(tee -a "$LOG_FILE") 2>&1

echo "[INFO] Starting Docker and Docker Compose setup..."

# Step 1: Update system packages
echo "[STEP] Updating system packages"
sudo dnf update -y

# Step 2: Install Docker Engine
echo "[STEP] Installing Docker Engine"
sudo dnf install -y docker

# Step 3: Enable and start Docker service
echo "[STEP] Enabling and starting Docker service"
sudo systemctl enable docker
sudo systemctl start docker

# Step 4: Add ec2-user to the docker group
echo "[STEP] Configuring permissions for ec2-user"
sudo usermod -aG docker ec2-user

# Step 5: Install Docker Compose
DOCKER_COMPOSE_VERSION="v2.39.2"
echo "[STEP] Installing Docker Compose (version ${DOCKER_COMPOSE_VERSION})"
sudo curl -L \
  "https://github.com/docker/compose/releases/download/${DOCKER_COMPOSE_VERSION}/docker-compose-$(uname -s)-$(uname -m)" \
  -o /usr/local/bin/docker-compose

sudo chmod +x /usr/local/bin/docker-compose

# Step 6: Verify installation
echo "[CHECK] Verifying Docker and Docker Compose installation"
docker --version || { echo "[ERROR] Docker installation failed"; exit 1; }
docker-compose --version || { echo "[ERROR] Docker Compose installation failed"; exit 1; }

# Completed
echo "[SUCCESS] Docker and Docker Compose installed successfully!"
echo "[NOTE] You may need to log out and log back in to use 'docker' without sudo."
