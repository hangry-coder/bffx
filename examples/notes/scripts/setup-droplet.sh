#!/bin/bash
set -e

echo "📦 Installing Docker..."
apt-get update -qq
apt-get install -y ca-certificates curl gnupg lsb-release
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker.gpg] \
  https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" > /etc/apt/sources.list.d/docker.list
apt-get update -qq
apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

echo "📁 Creating app directory..."
mkdir -p /var/www/bffx-app

echo "✅ Droplet ready! Configure GitHub Secrets and run: bffx deploy ship"
