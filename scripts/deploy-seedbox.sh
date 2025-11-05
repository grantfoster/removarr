#!/bin/bash

# Removarr Seedbox Deployment Script (No Root Required)
# This script helps you deploy Removarr to a seedbox without root access

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
INSTALL_DIR="$HOME/removarr"

echo "🚀 Removarr Seedbox Deployment"
echo "================================"
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed."
    echo "   Please install Go first: https://go.dev/doc/install"
    echo "   Or use a pre-built binary if available."
    exit 1
fi

echo "✅ Go found: $(go version)"
echo ""

# Create installation directory
echo "📁 Creating installation directory..."
mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"

# Copy source files (if not already there)
if [ ! -f "$INSTALL_DIR/go.mod" ]; then
    echo "📦 Copying source files..."
    cp -r "$PROJECT_DIR"/* "$INSTALL_DIR/" 2>/dev/null || {
        echo "⚠️  Could not copy files. Assuming we're in the project directory."
        INSTALL_DIR="$PROJECT_DIR"
    }
fi

cd "$INSTALL_DIR"

# Build binary
echo "🔨 Building Removarr binary..."
if go build -o removarr ./cmd/removarr; then
    echo "✅ Binary built successfully!"
else
    echo "❌ Build failed. Check errors above."
    exit 1
fi

# Build migrate tool
echo "🔨 Building migration tool..."
if go build -o migrate ./cmd/migrate; then
    echo "✅ Migration tool built successfully!"
else
    echo "❌ Build failed. Check errors above."
    exit 1
fi

# Create config file if it doesn't exist
if [ ! -f "config.yaml" ]; then
    echo "📝 Creating config.yaml..."
    if [ -f "config.example.yaml" ]; then
        cp config.example.yaml config.yaml
        echo "✅ Created config.yaml from example"
        echo ""
        echo "⚠️  IMPORTANT: Edit config.yaml with your database settings!"
        echo "   Database connection is required before running migrations."
    else
        echo "⚠️  config.example.yaml not found. Creating minimal config..."
        cat > config.yaml <<EOF
server:
  host: "0.0.0.0"
  port: 8080
  base_url: "http://localhost:8080"
  session_secret: ""
  session_max_age: "168h"

database:
  host: "localhost"
  port: 5432
  user: "removarr"
  password: ""
  database: "removarr"
  ssl_mode: "disable"

logging:
  level: "info"
  format: "json"
  file: ""
EOF
        echo "✅ Created minimal config.yaml"
        echo ""
        echo "⚠️  IMPORTANT: Edit config.yaml with your database settings!"
    fi
else
    echo "✅ config.yaml already exists"
fi

echo ""
echo "📋 Next Steps:"
echo "=============="
echo ""
echo "1. Edit config.yaml with your database connection:"
echo "   nano $INSTALL_DIR/config.yaml"
echo ""
echo "2. Create database (if needed):"
echo "   psql -h your_host -U your_user -d postgres"
echo "   CREATE DATABASE removarr;"
echo ""
echo "3. Run migrations:"
echo "   cd $INSTALL_DIR"
echo "   ./migrate -config config.yaml -cmd up"
echo ""
echo "4. Start Removarr:"
echo "   # Using screen (recommended):"
echo "   screen -dmS removarr $INSTALL_DIR/removarr -config $INSTALL_DIR/config.yaml"
echo ""
echo "   # Or using nohup:"
echo "   nohup $INSTALL_DIR/removarr -config $INSTALL_DIR/config.yaml > removarr.log 2>&1 &"
echo ""
echo "5. Access Removarr:"
echo "   http://your-seedbox-ip:8080"
echo ""
echo "6. Complete setup wizard and configure integrations in Settings"
echo ""
echo "📚 For more details, see DEPLOYMENT.md"
echo ""
echo "✅ Deployment files ready in: $INSTALL_DIR"

