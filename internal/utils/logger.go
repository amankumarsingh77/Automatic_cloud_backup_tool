package utils

import (
	"log"
	"os"
)

var (
	INFO    = "INFO"
	WARNING = "WARNING"
	ERROR   = "ERROR"
)

type Logger struct {
	infoLogger    *log.Logger
	warningLogger *log.Logger
	errorLogger   *log.Logger
}

func NewLogger() *Logger {
	return &Logger{
		infoLogger:    log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime|log.Lshortfile),
		warningLogger: log.New(os.Stdout, "WARNING\t", log.Ldate|log.Ltime|log.Lshortfile),
		errorLogger:   log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile),
	}
}

func (l *Logger) Log(format string, mssg string) {
	switch format {
	case INFO:
		l.infoLogger.Println(mssg)
	case WARNING:
		l.warningLogger.Fatal(mssg)
	case ERROR:
		l.errorLogger.Fatal(mssg)
	}
}
