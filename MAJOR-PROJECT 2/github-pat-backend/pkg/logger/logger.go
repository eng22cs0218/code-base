package logger

import (
	"fmt"
	"log"
	"os"
	"time"
)

type Logger struct {
	serviceName string
	logger      *log.Logger
}

func New(serviceName string) *Logger {
	return &Logger{
		serviceName: serviceName,
		logger:      log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (l *Logger) Info(msg string, fields ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	l.logger.Printf("[%s] [%s] INFO: %s %v", timestamp, l.serviceName, msg, fields)
}

func (l *Logger) Error(msg string, err error, fields ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	if err != nil {
		l.logger.Printf("[%s] [%s] ERROR: %s - %v %v", timestamp, l.serviceName, msg, err, fields)
	} else {
		l.logger.Printf("[%s] [%s] ERROR: %s %v", timestamp, l.serviceName, msg, fields)
	}
}

func (l *Logger) Debug(msg string, fields ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	l.logger.Printf("[%s] [%s] DEBUG: %s %v", timestamp, l.serviceName, msg, fields)
}

func (l *Logger) Warn(msg string, fields ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	l.logger.Printf("[%s] [%s] WARN: %s %v", timestamp, l.serviceName, msg, fields)
}

func (l *Logger) Fatal(msg string, err error) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	l.logger.Fatalf("[%s] [%s] FATAL: %s - %v", timestamp, l.serviceName, msg, err)
}

func (l *Logger) Infof(format string, args ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	l.logger.Printf("[%s] [%s] INFO: %s", timestamp, l.serviceName, msg)
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	l.logger.Printf("[%s] [%s] ERROR: %s", timestamp, l.serviceName, msg)
}
