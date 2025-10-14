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
  echo "[!] llama.cpp not found. Run: git submodule update --init" >&2
  exit 1
fi

mkdir -p "$LLAMA_BUILD"
cd "$LLAMA_BUILD"
cmake .. -DLLAMA_CUDA=ON -DCMAKE_BUILD_TYPE=Release
cmake --build . --target llama-cli -j"$(nproc)"
cp bin/llama-cli "$BIN_DIR/llama-cli-cuda"

# --- whisper.cpp (CUDA) ---
echo "Building whisper.cpp (CUDA)..."
WHISPER_DIR="$THIRD_PARTY/whisper.cpp"
WHISPER_BUILD="$WHISPER_DIR/build-cuda"

if [[ ! -f "$WHISPER_DIR/CMakeLists.txt" ]]; then
  echo "Error: whisper.cpp not found. Run: git submodule update --init" >&2
  exit 1
fi

mkdir -p "$WHISPER_BUILD"
cd "$WHISPER_BUILD"
cmake .. -DWHISPER_BUILD_CLI=ON -DGGML_CUDA=ON -DCMAKE_BUILD_TYPE=Release
cmake --build . --target whisper-cli -j"$(nproc)"
cp bin/whisper-cli "$BIN_DIR/whisper-cli-cuda"

echo "[+] All third-party binaries built with CUDA support."