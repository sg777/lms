#!/bin/bash
# Build script for lms-service

echo "Building lms-service..."

# Pull latest code (optional - comment out if you don't want auto-pull)
if [ -d .git ]; then
    echo "Pulling latest code..."
    git pull
fi

# Build
go build -o lms-service ./main.go
if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
    ls -lh lms-service
else
    echo "❌ Build failed!"
    exit 1
fi

