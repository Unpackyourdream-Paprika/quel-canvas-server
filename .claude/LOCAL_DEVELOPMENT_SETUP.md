# Local Development Setup Guide

## Overview
This guide helps you set up the local development environment for the Quel Canvas Server, which requires CGO (C compiler) for WebP image processing.

---

## Prerequisites
- Go 1.21+
- Git
- Redis (running locally on port 6379)

---

## Windows Setup

### 1. Install MSYS2
1. Download and install MSYS2 from https://www.msys2.org/
2. Run the installer and follow the default installation steps
3. Open **MSYS2 MSYS** terminal from Start menu

### 2. Install Required Packages
In MSYS2 terminal, run the following commands:

```bash
# Update package database
pacman -Syu

# Install MinGW-w64 toolchain (includes gcc)
pacman -S mingw-w64-x86_64-toolchain
# Press Enter to install all packages (default=all)

# Install libwebp library
pacman -S mingw-w64-x86_64-libwebp
```

### 3. Add to System PATH
1. Open Windows Start Menu → Search "환경 변수" (Environment Variables)
2. Click "시스템 환경 변수 편집" (Edit system environment variables)
3. Click "환경 변수" (Environment Variables) button
4. In System variables, find and select **"Path"**
5. Click "편집" (Edit)
6. Click "새로 만들기" (New)
7. Add: `C:\msys64\mingw64\bin`
8. Click OK to save

### 4. Configure Git Bash Environment

**Option A: Temporary (per session)**
```bash
export CGO_ENABLED=1
export CC=gcc
export PATH="/c/msys64/mingw64/bin:$PATH"
export PKG_CONFIG_PATH=/c/msys64/mingw64/lib/pkgconfig
```

**Option B: Permanent (recommended)**

Edit `~/.bashrc`:
```bash
nano ~/.bashrc
```

Add the following lines:
```bash
# CGO Configuration for WebP support
export CGO_ENABLED=1
export CC=gcc
export PATH="/c/msys64/mingw64/bin:$PATH"
export PKG_CONFIG_PATH=/c/msys64/mingw64/lib/pkgconfig
```

Save and restart your terminal.

### 5. Configure Local Environment Variables

Edit `.env` file in project root for local development:

```env
# Redis Configuration (Local)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_USERNAME=
REDIS_PASSWORD=
REDIS_USE_TLS=false

# Supabase Configuration
SUPABASE_URL=https://ftunegfzqsbhtucctaqq.supabase.co
SUPABASE_SERVICE_KEY=your_service_key_here

# Gemini Configuration
GEMINI_API_KEY=your_gemini_api_key_here
GEMINI_MODEL=gemini-2.5-flash-image

# Server Configuration
PORT=8080
CREDIT_PER_IMAGE=20
```

**Note:** Production uses different Redis settings with TLS enabled. Render environment variables override `.env` file in production.

### 6. Run the Server

```bash
cd ~/Desktop/server/quel/quel-canvas-server
go run main.go
```

Expected output:
```
✅ Configuration loaded successfully
   Redis: localhost:6379 (TLS: false)
   Supabase: https://ftunegfzqsbhtucctaqq.supabase.co
   Gemini: gemini-2.5-flash-image
🚀 Quel Canvas Collaboration Server starting on port 8080
📡 WebSocket endpoint: ws://localhost:8080/ws
❤️  Health check: http://localhost:8080/health
✅ Redis connected successfully
```

---

## macOS Setup

### 1. Install Homebrew (if not already installed)
```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

### 2. Install Required Packages
```bash
# Install webp library
brew install webp

# Install pkg-config (for finding libraries)
brew install pkg-config
```

### 3. Configure Environment Variables

**Option A: Temporary (per session)**
```bash
export CGO_ENABLED=1
export PKG_CONFIG_PATH="$(brew --prefix)/lib/pkgconfig"
```

**Option B: Permanent (recommended)**

For **zsh** (default on macOS Catalina+):
```bash
nano ~/.zshrc
```

For **bash**:
```bash
nano ~/.bash_profile
```

Add the following lines:
```bash
# CGO Configuration for WebP support
export CGO_ENABLED=1
export PKG_CONFIG_PATH="$(brew --prefix)/lib/pkgconfig"
```

Save and restart your terminal or run:
```bash
source ~/.zshrc  # for zsh
# or
source ~/.bash_profile  # for bash
```

### 4. Configure Local Environment Variables

Same as Windows - edit `.env` file:

```env
# Redis Configuration (Local)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_USERNAME=
REDIS_PASSWORD=
REDIS_USE_TLS=false

# ... rest of configuration
```

### 5. Install and Run Redis (if not already running)

```bash
# Install Redis
brew install redis

# Start Redis service
brew services start redis

# Or run Redis manually
redis-server
```

### 6. Run the Server

```bash
cd ~/Desktop/server/quel/quel-canvas-server
go run main.go
```

---

## Troubleshooting

### Windows: "gcc not found" error
- Make sure `C:\msys64\mingw64\bin` is in your System PATH
- Restart Git Bash terminal after adding to PATH
- Verify gcc is accessible: `which gcc` should output `/c/msys64/mingw64/bin/gcc`

### Windows: "build constraints exclude all Go files"
- This means CGO is not enabled or gcc is not found
- Run `export CGO_ENABLED=1` in your terminal
- Make sure gcc is in PATH

### macOS: "library not found" error
- Make sure webp is installed: `brew list webp`
- Check PKG_CONFIG_PATH: `echo $PKG_CONFIG_PATH`
- Reinstall webp if needed: `brew reinstall webp`

### Redis connection error
- Make sure Redis is running: `redis-cli ping` (should return "PONG")
- Windows: Download Redis from https://github.com/tporadowski/redis/releases
- macOS: `brew services start redis`
- Check `.env` has correct Redis settings for local

### "invalid input syntax for type uuid" in logs
- This is normal when using `/test-redis` API endpoint
- Test jobs use timestamp-based IDs, not UUIDs
- Real production jobs from frontend use proper UUIDs

---

## Dependency Information

### WebP Library
- **Package:** `github.com/kolesa-team/go-webp`
- **Why CGO is needed:** This package wraps the native libwebp C library for optimal performance
- **Alternative:** Pure Go implementations exist but are slower and produce larger files

### Redis Configuration
- **Local:** Plain connection without TLS (`REDIS_USE_TLS=false`)
- **Production (Render):** TLS-encrypted connection (`REDIS_USE_TLS=true`)
- **Why different:** Local traffic stays on same machine (safe), production traffic goes over internet (needs encryption)

---

## Verification

### Check CGO is working:
```bash
go env CGO_ENABLED
# Should output: 1
```

### Check gcc is accessible:
```bash
gcc --version
# Should show gcc version information
```

### Check Redis is running:
```bash
redis-cli ping
# Should output: PONG
```

### Test the server:
```bash
curl http://localhost:8080/health
# Should return: {"status":"ok"}
```

---

## Notes

- `.env` file is for local development only
- Production uses Render environment variables (not `.env` file)
- Never commit `.env` file with real credentials to git
- CGO must be enabled for WebP encoding/decoding to work
