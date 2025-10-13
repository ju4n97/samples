#!/usr/bin/env bash
# Builds all third-party C/C++ backends for CUDA

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
THIRD_PARTY="$ROOT/third_party"
BIN_DIR="$ROOT/bin"

mkdir -p "$BIN_DIR"

# --- llama.cpp ---
echo "Building llama.cpp (CUDA)..."
LLAMA_DIR="$THIRD_PARTY/llama.cpp"
LLAMA_BUILD="$LLAMA_DIR/build-cuda"

if [[ ! -f "$LLAMA_DIR/CMakeLists.txt" ]]; then
  echo "Error: llama.cpp not found. Run: git submodule update --init" >&2
  exit 1
fi

mkdir -p "$LLAMA_BUILD"
cd "$LLAMA_BUILD"
cmake .. -DLLAMA_BUILD_CLI=ON -DLLAMA_CUDA=ON -DCMAKE_BUILD_TYPE=Release
cmake --build . --target llama-cli -j"$(nproc)"
cp bin/llama-cli "$BIN_DIR/llama-cli-cuda"

# --- whisper.cpp ---

echo "✅ All third-party CUDA binaries built."