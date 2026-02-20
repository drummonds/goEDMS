//go:build js && wasm

// For WASM builds, no SQL drivers are registered.
// The WASM demo uses MemDB (pure Go in-memory repository) instead.
package database
