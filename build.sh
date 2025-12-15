#!/bin/bash
# Build script for ALL LMS components

set -e  # Exit on any error

echo "=========================================="
echo "Building ALL LMS Components"
echo "=========================================="

# Pull latest code (optional - comment out if you don't want auto-pull)
if [ -d .git ]; then
    echo "Pulling latest code..."
    git pull || echo "Warning: git pull failed (not a git repo or no network)"
    echo "Initializing/updating Git submodules..."
    git submodule update --init --recursive || echo "Warning: submodule update failed"
fi

# Build hash-sigs library if needed
if [ ! -f native/hash-sigs/hss_lib_thread.a ]; then
    echo ""
    echo "Building hash-sigs library..."
    if [ -f native/hash-sigs/Makefile ]; then
        cd native/hash-sigs
        if make hss_lib_thread.a; then
            echo "   ✅ hash-sigs library built successfully!"
            cd ../..
        else
            echo "   ❌ hash-sigs library build failed!"
            echo "   Please ensure you have gcc, make, and OpenSSL development libraries installed"
            exit 1
        fi
    else
        echo "   ❌ Makefile not found in native/hash-sigs/"
        echo "   Please ensure the hash-sigs submodule is initialized: git submodule update --init --recursive"
        exit 1
    fi
else
    echo "   ✅ hash-sigs library already exists"
fi

BUILD_ERRORS=0

# Build lms-service
echo ""
echo "1. Building lms-service..."
if go build -o lms-service ./main.go; then
    echo "   ✅ lms-service build successful!"
    ls -lh lms-service
else
    echo "   ❌ lms-service build failed!"
    BUILD_ERRORS=$((BUILD_ERRORS + 1))
fi

# Build explorer
echo ""
echo "2. Building lms-explorer..."
if go build -o lms-explorer ./cmd/explorer; then
    echo "   ✅ lms-explorer build successful!"
    ls -lh lms-explorer
else
    echo "   ❌ lms-explorer build failed!"
    BUILD_ERRORS=$((BUILD_ERRORS + 1))
fi

# Build hsm-server
echo ""
echo "3. Building hsm-server..."
if go build -o hsm-server ./cmd/hsm-server; then
    echo "   ✅ hsm-server build successful!"
    ls -lh hsm-server
else
    echo "   ❌ hsm-server build failed!"
    BUILD_ERRORS=$((BUILD_ERRORS + 1))
fi

# Build hsm-client
echo ""
echo "4. Building hsm-client..."
if go build -o hsm-client ./cmd/hsm-client; then
    echo "   ✅ hsm-client build successful!"
    ls -lh hsm-client
else
    echo "   ❌ hsm-client build failed!"
    BUILD_ERRORS=$((BUILD_ERRORS + 1))
fi

echo ""
echo "=========================================="
if [ $BUILD_ERRORS -eq 0 ]; then
    echo "✅ ALL builds successful!"
    echo ""
    echo "Built binaries:"
    ls -lh lms-service lms-explorer hsm-server hsm-client 2>/dev/null | awk '{print "  " $9 " (" $5 ")"}'
else
    echo "❌ $BUILD_ERRORS build(s) failed!"
    exit 1
fi
echo "=========================================="

