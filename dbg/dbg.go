// +build debug

package dbg

import (
	"flag"
	"fmt"
	"runtime"
	"strings"
)

var levelFlag *int = flag.Int("level", 0, "log leve: trace(0), debug(1), info(2), warn(3), error(4)")

func Trace(format string, args ...interface{}) {
	log(0, format, args...)
}

func Debug(format string, args ...interface{}) {
	log(1, format, args...)
}

func Info(fmt string, args ...interface{}) {
	log(2, fmt, args...)
}

func Warn(fmt string, args ...interface{}) {
	log(3, fmt, args...)
}

func Error(fmt string, args ...interface{}) {
	log(4, fmt, args...)
}

func log(level int, format string, args ...interface{}) {
	if levelFlag == nil {
		*levelFlag = 0
	}

	var prefix string
	switch level {
	case 0:
		prefix = "trace"
	case 1:
		prefix = "debug"
	case 2:
		prefix = "info"
	case 3:
		prefix = "warn"
	case 4:
		prefix = "error"
	}

	pc, _, line, _ := runtime.Caller(2)
	name := runtime.FuncForPC(pc).Name()
	parts := strings.Split(name, "/")

	fmt.Println("LEVEL FLKAG", *levelFlag)
	if level >= *levelFlag {
		msg := fmt.Sprintf(format, args...)
		fmt.Printf(
			"%s\n\t%s:%d\n\t\t%s\n",
			prefix,
			parts[len(parts)-1],
			line,
			msg,
		)
	}
}

func init() {
	flag.Parse()
}
