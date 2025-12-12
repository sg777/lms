#!/bin/bash
# Build script for lms-service

echo "Building lms-service..."
go build -o lms-service .
if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
    ls -lh lms-service
else
    echo "❌ Build failed!"
    exit 1
fi

