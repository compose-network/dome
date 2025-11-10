package logger

import (
	"log"
	"strings"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var currentLevel LogLevel = INFO

// SetLogLevel sets the current log level
func SetLogLevel(level LogLevel) {
	currentLevel = level
}

// SetLogLevelFromString sets log level from string
func SetLogLevelFromString(level string) {
	switch strings.ToLower(level) {
	case "debug":
		SetLogLevel(DEBUG)
	case "info":
		SetLogLevel(INFO)
	case "warn", "warning":
		SetLogLevel(WARN)
	case "error":
		SetLogLevel(ERROR)
	default:
		SetLogLevel(INFO)
	}
}

// Debug logs debug messages
func Debug(format string, v ...interface{}) {
	if currentLevel <= DEBUG {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// Info logs info messages
func Info(format string, v ...interface{}) {
	if currentLevel <= INFO {
		log.Printf("[INFO] "+format, v...)
	}
}

// Warn logs warning messages
func Warn(format string, v ...interface{}) {
	if currentLevel <= WARN {
		log.Printf("[WARN] "+format, v...)
	}
}

// Error logs error messages
func Error(format string, v ...interface{}) {
	if currentLevel <= ERROR {
		log.Printf("[ERROR] "+format, v...)
	}
}

// Fatal logs fatal messages and exits
func Fatal(format string, v ...interface{}) {
	log.Fatalf("[FATAL] "+format, v...)
}
