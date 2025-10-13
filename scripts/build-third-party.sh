#!/usr/bin/env bash
# Builds all third-party C/C++ backends for CPU

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
THIRD_PARTY="$ROOT/third_party"
BIN_DIR="$ROOT/bin"

mkdir -p "$BIN_DIR"

# --- llama.cpp ---
echo "Building llama.cpp (CPU)..."
LLAMA_DIR="$THIRD_PARTY/llama.cpp"
LLAMA_BUILD="$LLAMA_DIR/build-cpu"

if [[ ! -f "$LLAMA_DIR/CMakeLists.txt" ]]; then
  echo "[!] llama.cpp not found. Run: git submodule update --init" >&2
  exit 1
fi

mkdir -p "$LLAMA_BUILD"
cd "$LLAMA_BUILD"
cmake .. -DLLAMA_BUILD_CLI=ON -DLLAMA_CUDA=OFF -DCMAKE_BUILD_TYPE=Release
cmake --build . --target llama-cli -j"$(nproc)"
cp bin/llama-cli "$BIN_DIR/llama-cli-cpu"

# --- whisper.cpp ---
echo "Building whisper.cpp (CPU)..."
WHISPER_DIR="$THIRD_PARTY/whisper.cpp"
WHISPER_BUILD="$WHISPER_DIR/build-cpu"

if [[ ! -f "$WHISPER_DIR/CMakeLists.txt" ]]; then
  echo "[!] whisper.cpp not found. Run: git submodule update --init" >&2
  exit 1
fi

mkdir -p "$WHISPER_BUILD"
cd "$WHISPER_BUILD"
cmake .. -DWHISPER_BUILD_CLI=ON -DWHISPER_CUDA=OFF -DCMAKE_BUILD_TYPE=Release
cmake --build . --target whisper-cli -j"$(nproc)"
cp bin/whisper-cli "$BIN_DIR/whisper-cli-cpu"

# --- piper (CPU) ---
echo "Downloading piper (CPU)..."
PIPER_URL="https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_linux_x86_64.tar.gz"
mkdir -p /tmp/piper-download
curl -L "$PIPER_URL" | tar -xz -C /tmp/piper-download
cp -r /tmp/piper-download/piper "$BIN_DIR/piper-cpu"
chmod +x "$BIN_DIR/piper-cpu"

echo "[+] All third-party CPU binaries built."