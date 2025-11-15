#!/bin/bash
# Build Swiss Ephemeris library for local development

echo "🔨 Building Swiss Ephemeris library..."

cd eph/sweph/src

# Clean previous builds
make clean

# Build the library
make libswe.a

echo "✅ Swiss Ephemeris library built successfully"
echo "Library location: eph/sweph/src/libswe.a"
