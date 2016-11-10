// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package zap

import (
	"fmt"
	"os"
)

// For tests.
var _exit = os.Exit

// A Logger enables leveled, structured logging. All methods are safe for
// concurrent use.
type Logger interface {
	// Check the minimum enabled log level.
	Level() Level
	// Change the level of this logger, as well as all its ancestors and
	// descendants. This makes it easy to change the log level at runtime
	// without restarting your application.
	SetLevel(Level)

	// Create a child logger, and optionally add some context to that logger.
	With(...Field) Logger

	// Check returns a CheckedMessage if logging a message at the specified level
	// is enabled. It's a completely optional optimization; in high-performance
	// applications, Check can help avoid allocating a slice to hold fields.
	//
	// See CheckedMessage for an example.
	Check(Level, string) *CheckedMessage

	// Log a message at the given level. Messages include any context that's
	// accumulated on the logger, as well as any fields added at the log site.
	//
	// Calling Panic should panic() and calling Fatal should terminate the
	// process, but calling Log(PanicLevel, ...) or Log(FatalLevel, ...) should
	// not. It may not be possible for compatibility wrappers to comply with
	// this last part (e.g. the bark wrapper).
	Log(Level, string, ...Field)
	Debug(string, ...Field)
	Info(string, ...Field)
	Warn(string, ...Field)
	Error(string, ...Field)
	Panic(string, ...Field)
	Fatal(string, ...Field)
	// If the logger is in development mode (via the Development option), DFatal
	// logs at the Fatal level. Otherwise, it logs at the Error level.
	DFatal(string, ...Field)
}

type logger struct{ Meta }

// New constructs a logger that uses the provided encoder. By default, the
// logger will write Info logs or higher to standard out. Any errors during logging
// will be written to standard error.
//
// Options can change the log level, the output location, the initial fields
// that should be added as context, and many other behaviors.
func New(enc Encoder, options ...Option) Logger {
	return &logger{
		Meta: MakeMeta(enc, options...),
	}
}

func (log *logger) With(fields ...Field) Logger {
	clone := &logger{
		Meta: log.Meta.Clone(),
	}
	addFields(clone.Encoder, fields)
	return clone
}

func (log *logger) Check(lvl Level, msg string) *CheckedMessage {
	switch lvl {
	case PanicLevel, FatalLevel:
		// Panic and Fatal should always cause a panic/exit, even if the level
		// is disabled.
		break
	default:
		if lvl < log.Level() {
			return nil
		}
	}
	return NewCheckedMessage(log, lvl, msg)
}

func (log *logger) Log(lvl Level, msg string, fields ...Field) {
	log.log(lvl, msg, fields)
}

func (log *logger) Debug(msg string, fields ...Field) {
	log.log(DebugLevel, msg, fields)
}

func (log *logger) Info(msg string, fields ...Field) {
	log.log(InfoLevel, msg, fields)
}

func (log *logger) Warn(msg string, fields ...Field) {
	log.log(WarnLevel, msg, fields)
}

func (log *logger) Error(msg string, fields ...Field) {
	log.log(ErrorLevel, msg, fields)
}

func (log *logger) Panic(msg string, fields ...Field) {
	log.log(PanicLevel, msg, fields)
	panic(msg)
}

func (log *logger) Fatal(msg string, fields ...Field) {
	log.log(FatalLevel, msg, fields)
	_exit(1)
}

func (log *logger) DFatal(msg string, fields ...Field) {
	if log.Development {
		log.Fatal(msg, fields...)
		return
	}
	log.Error(msg, fields...)
}

func (log *logger) log(lvl Level, msg string, fields []Field) {
	if !(lvl >= log.Level()) {
		return
	}

	temp := log.Encoder.Clone()
	addFields(temp, fields)

	entry := newEntry(lvl, msg, temp)
	for _, hook := range log.Hooks {
		if err := hook(entry); err != nil {
			log.internalError(err.Error())
		}
	}

	if err := temp.WriteEntry(log.Output, entry.Message, entry.Level, entry.Time); err != nil {
		log.internalError(err.Error())
	}
	temp.Free()
	entry.free()

	if lvl > ErrorLevel {
		// Sync on Panic and Fatal, since they may crash the program.
		log.Output.Sync()
	}
}

func (log *logger) internalError(msg string) {
	fmt.Fprintln(log.ErrorOutput, msg)
	log.ErrorOutput.Sync()
}
