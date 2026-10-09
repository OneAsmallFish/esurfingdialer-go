package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	mu     sync.RWMutex
	level  = LevelInfo
	logger = log.New(os.Stdout, "", log.LstdFlags)
)

func Configure(levelText string) {
	if levelText == "" {
		return
	}
	lvl, ok := parseLevel(levelText)
	if !ok {
		return
	}
	mu.Lock()
	level = lvl
	mu.Unlock()
}

func Debugf(format string, args ...any) {
	logf(LevelDebug, "DEBUG", format, args...)
}

func Infof(format string, args ...any) {
	logf(LevelInfo, "INFO", format, args...)
}

func Warnf(format string, args ...any) {
	logf(LevelWarn, "WARN", format, args...)
}

func Errorf(format string, args ...any) {
	logf(LevelError, "ERROR", format, args...)
}

func logf(lvl Level, prefix, format string, args ...any) {
	mu.RLock()
	cur := level
	mu.RUnlock()
	if lvl < cur {
		return
	}
	logger.Printf("[%s] %s", prefix, fmt.Sprintf(format, args...))
}

func parseLevel(text string) (Level, bool) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "debug":
		return LevelDebug, true
	case "info":
		return LevelInfo, true
	case "warn", "warning":
		return LevelWarn, true
	case "error":
		return LevelError, true
	default:
		return LevelInfo, false
	}
}
