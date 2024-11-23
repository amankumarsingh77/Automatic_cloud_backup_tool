package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARNING
	ERROR
	FATAL
)

var levelNames = map[LogLevel]string{
	DEBUG:   "DEBUG",
	INFO:    "INFO",
	WARNING: "WARNING",
	ERROR:   "ERROR",
	FATAL:   "FATAL",
}

type Logger struct {
	mu       sync.Mutex
	logFile  *os.File
	loggers  map[LogLevel]*log.Logger
	minLevel LogLevel
}

var (
	defaultLogger *Logger
	once         sync.Once
)

func GetLogger() *Logger {
	once.Do(func() {
		var err error
		defaultLogger, err = NewLogger("backup_service.log", INFO)
		if err != nil {
			log.Fatalf("Failed to initialize logger: %v", err)
		}
	})
	return defaultLogger
}

func NewLogger(filename string, minLevel LogLevel) (*Logger, error) {
	
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %v", err)
	}

	
	logPath := filepath.Join(logsDir, time.Now().Format("2006-01-02_")+filename)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	l := &Logger{
		logFile:  logFile,
		loggers:  make(map[LogLevel]*log.Logger),
		minLevel: minLevel,
	}

	
	for level, name := range levelNames {
		prefix := fmt.Sprintf("%s\t", name)
		flags := log.Ldate | log.Ltime | log.Lshortfile
		l.loggers[level] = log.New(logFile, prefix, flags)
	}

	return l, nil
}

func (l *Logger) log(level LogLevel, format string, v ...interface{}) {
	if level < l.minLevel {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "unknown"
		line = 0
	}

	
	msg := fmt.Sprintf(format, v...)
	
	
	logger := l.loggers[level]
	logger.Printf("[%s:%d] %s", filepath.Base(file), line, msg)

	
	if level >= ERROR {
		fmt.Fprintf(os.Stderr, "%s [%s:%d] %s\n", levelNames[level], filepath.Base(file), line, msg)
	}

	
	if level == FATAL {
		os.Exit(1)
	}
}

func (l *Logger) Debug(format string, v ...interface{}) {
	l.log(DEBUG, format, v...)
}

func (l *Logger) Info(format string, v ...interface{}) {
	l.log(INFO, format, v...)
}

func (l *Logger) Warning(format string, v ...interface{}) {
	l.log(WARNING, format, v...)
}

func (l *Logger) Error(format string, v ...interface{}) {
	l.log(ERROR, format, v...)
}

func (l *Logger) Fatal(format string, v ...interface{}) {
	l.log(FATAL, format, v...)
}

func (l *Logger) Close() error {
	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}
