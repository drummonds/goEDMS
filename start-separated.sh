#!/bin/bash

# Startup script for godocs separated architecture
# Backend (port 8000): API + static files
# Frontend (port 8001): HTML shell that loads from backend

set -e

echo "=================================================="
echo "🚀  Starting godocs in Separated Mode"
echo "=================================================="
echo "• Backend (API + Static): http://localhost:8000"
echo "• Frontend (HTML Shell):  http://localhost:8001"
echo "=================================================="
echo ""

# Check if WASM is built
if [ ! -f "web/app.wasm" ]; then
    echo "⚠️  WASM not found. Building..."
    mkdir -p web
    GOOS=js GOARCH=wasm go build -o web/app.wasm ./cmd/webapp
    echo "✅  WASM built successfully"
    echo ""
fi

# Start backend in background
echo "🔧  Starting backend server (port 8000)..."
go run ./cmd/backend/main.go -port 8000 &
BACKEND_PID=$!
echo "   Backend PID: $BACKEND_PID"

# Wait a moment for backend to start
sleep 2

# Start frontend in background
echo "🎨  Starting frontend server (port 8001)..."
go run ./cmd/frontend/main.go -port 8001 -backend http://localhost:8000 &
FRONTEND_PID=$!
echo "   Frontend PID: $FRONTEND_PID"

echo ""
echo "=================================================="
echo "✅  Both servers started!"
echo "=================================================="
echo "📱  Open http://localhost:8001 in your browser"
echo "📡  Backend API: http://localhost:8000/api/"
echo "📦  Static files: http://localhost:8000/web/"
echo ""
echo "Press Ctrl+C to stop both servers"
echo "=================================================="
echo ""

# Function to cleanup on exit
cleanup() {
    echo ""
    echo "🛑  Stopping servers..."
    kill $BACKEND_PID 2>/dev/null || true
    kill $FRONTEND_PID 2>/dev/null || true
    echo "✅  Servers stopped"
    exit 0
}

# Trap Ctrl+C and call cleanup
trap cleanup INT TERM

# Wait for both processes
wait
