// Package logger provides a very small, dependency-free logging facade
// used across the application for simple info/debug/error output.
package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
)

var std = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)

// Init initializes the logger. Level is currently informational only ("debug" prints debug messages).
func Init(level string) {
	// keep simple: no external deps, but allow prefix by level
	std.SetPrefix("")
	if strings.ToLower(level) == "debug" {
		// include a DEBUG prefix when printing debug messages
		std.SetPrefix("DEBUG ")
	}
}

func formatKV(msg string, kv ...interface{}) string {
	if len(kv) == 0 {
		return msg
	}
	var parts []string
	for i := 0; i+1 < len(kv); i += 2 {
		parts = append(parts, fmt.Sprintf("%v=%v", kv[i], kv[i+1]))
	}
	// if odd number of items, append the last
	if len(kv)%2 == 1 {
		parts = append(parts, fmt.Sprintf("%v", kv[len(kv)-1]))
	}
	return fmt.Sprintf("%s %s", msg, strings.Join(parts, " "))
}

// Infof formats and logs an informational message.
func Infof(format string, args ...interface{}) {
	std.Printf("INFO "+format+"\n", args...)
}

// Infow logs an informational message with key/value pairs.
func Infow(msg string, kv ...interface{}) {
	std.Printf("INFO %s\n", formatKV(msg, kv...))
}

// Errorf formats and logs an error message.
func Errorf(format string, args ...interface{}) {
	std.Printf("ERROR "+format+"\n", args...)
}

// Errorw logs an error message with key/value pairs.
func Errorw(msg string, kv ...interface{}) {
	std.Printf("ERROR %s\n", formatKV(msg, kv...))
}

// Debugf formats and logs a debug message.
func Debugf(format string, args ...interface{}) {
	std.Printf("DEBUG "+format+"\n", args...)
}

// Debugw logs a debug message with key/value pairs.
func Debugw(msg string, kv ...interface{}) {
	std.Printf("DEBUG %s\n", formatKV(msg, kv...))
}

// Sync is a no-op for the simple logger (kept for API compatibility).
func Sync() error { return nil }
