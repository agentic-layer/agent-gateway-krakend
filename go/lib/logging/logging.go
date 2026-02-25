package logging

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Logger matches the interface KrakenD provides via RegisterLogger.
type Logger interface {
	Debug(v ...interface{})
	Info(v ...interface{})
	Warning(v ...interface{})
	Error(v ...interface{})
	Critical(v ...interface{})
	Fatal(v ...interface{})
}

// fallbackLogger implements Logger using fmt, producing output in KrakenD's format.
type fallbackLogger struct {
	pluginName string
}

func (l *fallbackLogger) log(level string, v ...interface{}) {
	ts := time.Now().Format("2006/01/02 - 15:04:05.000")
	fmt.Fprintf(os.Stdout, " %s ▶ %-5s [%s] %s\n", ts, level, strings.ToUpper(l.pluginName), fmt.Sprint(v...))
}

func (l *fallbackLogger) Debug(v ...interface{})    { l.log("DEBUG", v...) }
func (l *fallbackLogger) Info(v ...interface{})     { l.log("INFO", v...) }
func (l *fallbackLogger) Warning(v ...interface{})  { l.log("WARN", v...) }
func (l *fallbackLogger) Error(v ...interface{})    { l.log("ERROR", v...) }
func (l *fallbackLogger) Critical(v ...interface{}) { l.log("CRIT", v...) }
func (l *fallbackLogger) Fatal(v ...interface{})    { l.log("FATAL", v...) }

// New returns a Logger that produces output in KrakenD's log format.
// Used as a fallback before KrakenD provides its own logger via RegisterLogger.
func New(pluginName string) Logger {
	return &fallbackLogger{pluginName: pluginName}
}

// prefixLogger wraps a KrakenD-provided logger and prepends the plugin name tag to every message.
type prefixLogger struct {
	inner      Logger
	pluginName string
}

func (l *prefixLogger) tag(v []interface{}) []interface{} {
	return append([]interface{}{fmt.Sprintf("[%s]", strings.ToUpper(l.pluginName))}, v...)
}

func (l *prefixLogger) Debug(v ...interface{})    { l.inner.Debug(l.tag(v)...) }
func (l *prefixLogger) Info(v ...interface{})     { l.inner.Info(l.tag(v)...) }
func (l *prefixLogger) Warning(v ...interface{})  { l.inner.Warning(l.tag(v)...) }
func (l *prefixLogger) Error(v ...interface{})    { l.inner.Error(l.tag(v)...) }
func (l *prefixLogger) Critical(v ...interface{}) { l.inner.Critical(l.tag(v)...) }
func (l *prefixLogger) Fatal(v ...interface{})    { l.inner.Fatal(l.tag(v)...) }

// Wrap adapts a KrakenD-provided logger to our Logger interface,
// prepending the plugin name tag to every message.
func Wrap(v interface{}, pluginName string) (Logger, bool) {
	l, ok := v.(Logger)
	if !ok {
		return nil, false
	}
	return &prefixLogger{inner: l, pluginName: pluginName}, true
}
