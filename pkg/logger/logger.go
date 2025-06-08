package logger

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/fatih/color"
)

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

type Logger struct {
	name      string
	level     LogLevel
	out       io.Writer
	color     bool
	formatter func(level LogLevel, message string) string
}

var log = &Logger{
	name:      "app",
	level:     LevelInfo,
	out:       os.Stdout,
	color:     true,
	formatter: defaultFormatter,
}

func Debug(msg string, args ...any) { log.log(LevelDebug, msg, args...) }
func Info(msg string, args ...any)  { log.log(LevelInfo, msg, args...) }
func Warn(msg string, args ...any)  { log.log(LevelWarn, msg, args...) }
func Error(msg string, args ...any) { log.log(LevelError, msg, args...) }
func Fatal(msg string, args ...any) { log.log(LevelFatal, msg, args...); os.Exit(1) }

func SetLevel(level LogLevel)                       { log.level = level }
func SetOutput(out io.Writer)                       { log.out = out }
func SetColor(enabled bool)                         { log.color = enabled }
func SetFormatter(fn func(LogLevel, string) string) { log.formatter = fn }

func (l *Logger) log(level LogLevel, msg string, args ...any) {
	if level < l.level {
		return
	}

	formattedMsg := msg
	if len(args) > 0 {
		formattedMsg = fmt.Sprintf(msg, args...)
	}

	output := l.formatter(level, formattedMsg)
	if !l.color {
		output = stripColors(output)
	}

	fmt.Fprintln(l.out, output)
}

func defaultFormatter(level LogLevel, message string) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, file, line, _ := runtime.Caller(3)
	fileParts := strings.Split(file, "/")
	if len(fileParts) > 3 {
		fileParts = fileParts[len(fileParts)-3:]
	}
	caller := fmt.Sprintf("%s:%d", strings.Join(fileParts, "/"), line)

	var levelStr string
	var levelColor *color.Color
	var messageColor *color.Color

	switch level {
	case LevelDebug:
		levelStr = "DEBUG"
		levelColor = color.New(color.FgHiCyan)
		messageColor = color.New(color.FgCyan)
	case LevelInfo:
		levelStr = "INFO"
		levelColor = color.New(color.FgHiGreen)
		messageColor = color.New(color.FgGreen)
	case LevelWarn:
		levelStr = "WARN"
		levelColor = color.New(color.FgHiYellow)
		messageColor = color.New(color.FgYellow)
	case LevelError:
		levelStr = "ERROR"
		levelColor = color.New(color.FgHiRed)
		messageColor = color.New(color.FgRed)
	case LevelFatal:
		levelStr = "FATAL"
		levelColor = color.New(color.FgHiMagenta)
		messageColor = color.New(color.FgMagenta)
	}

	return fmt.Sprintf("%s | %s | %s | %s",
		color.New(color.FgHiWhite).Sprint(timestamp),
		levelColor.Sprintf("%-5s", levelStr),
		color.New(color.FgHiBlue).Sprint(caller),
		messageColor.Sprint(message))
}

func stripColors(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*[mGK]`)
	return re.ReplaceAllString(s, "")
}
