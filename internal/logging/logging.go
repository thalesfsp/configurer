// Copyright 2021 The authors. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package logging

import (
	"github.com/thalesfsp/sypl"
	"github.com/thalesfsp/sypl/level"
	"github.com/thalesfsp/sypl/processor"
)

//////
// Vars, consts, and types.
//////

// Singleton.
var singletonLogger *Logger

// Logger is the application logger.
type Logger struct {
	*sypl.Sypl
}

// Child creates a new child logger.
func (l *Logger) Child(name string) *Logger {
	return &Logger{
		Sypl: l.New(name),
	}
}

//////
// Exported functionalities.
//////

// Get returns a setup logger, or set it up. Default level is `NONE`.
//
// All messages will be directed to:
// - StdOut
// - StdErr - In case of ERROR level
//
// NOTE: Use `SYPL_DEBUG` env var to overwrite the max level.
// SEE: https://github.com/thalesfsp/sypl/blob/master/CHANGELOG.md#154---2021-10-13
func Get() *Logger {
	if singletonLogger == nil {
		singletonLogger = &Logger{
			sypl.NewDefault("configurer", level.None),
		}

		// Iterate over the outputs and add the lower case processor.
		for _, o := range singletonLogger.GetOutputs() {
			o.AddProcessors(processor.ChangeFirstCharCase(processor.Lowercase))
		}

		singletonLogger.Traceln("configurer logger setup")

		return singletonLogger
	}

	return singletonLogger
}
