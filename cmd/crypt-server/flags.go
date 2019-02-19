package cryptserver

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/kurafs/kura/pkg/log"
)

type logMode struct {
	m   log.Mode
	set bool
}

func (l logMode) String() string {
	return modeToString(log.Mode(l.m))
}

func (l *logMode) Set(value string) error {
	l.set = true

	m, err := modeFromString(value)
	if err != nil {
		return err
	}
	l.m = m
	return nil
}

type fileLogMode struct {
	fname string
	fmode log.Mode
}
type logFilter []fileLogMode

func (l logFilter) String() string {
	var buf bytes.Buffer
	buf.WriteString("[")
	for i := 0; i < len(l)-1; i++ {
		buf.WriteString(fmt.Sprintf("%s:%s ", l[i].fname, modeToString(l[i].fmode)))
	}

	if len(l) > 0 {
		lastIndex := len(l) - 1
		buf.WriteString(fmt.Sprintf("%s:%s", l[lastIndex].fname, modeToString(l[lastIndex].fmode)))
	}
	buf.WriteString("]")
	return buf.String()
}

func (l *logFilter) Set(value string) error {
	fileNameRegex := "^[\\w(\\-)*]+.go$"
	modeRegex := "^[info|debug|warn|error][\\|(info|debug|warn|error)]*$"

	for _, f := range strings.Split(value, ",") {
		f := strings.Split(f, ":")
		if len(f) != 2 {
			return errors.New(
				fmt.Sprintf("Improperly formatted filter: %s, expected fname.go:mode", f))
		}

		fname, mode := f[0], f[1]
		matched, err := regexp.Match(fileNameRegex, []byte(fname))
		if err != nil {
			return err
		}
		if !matched {
			// TODO(irfansharif): Better error here.
			return errors.New(
				fmt.Sprintf("Expected filename '%s' to match the regex '%s'", fname, fileNameRegex))
		}

		matched, err = regexp.Match(modeRegex, []byte(mode))
		if err != nil {
			return err
		}
		if !matched {
			return errors.New(
				fmt.Sprintf("Expected mode '%s' to match the regex '%s'", mode, modeRegex))
		}

		fmode, err := modeFromString(mode)
		if err != nil {
			return err
		}
		*l = append(*l, fileLogMode{fname: fname, fmode: fmode})
	}
	return nil
}

type backtracePoints []string

func (l *backtracePoints) String() string {
	return fmt.Sprint(*l)
}

func (l *backtracePoints) Set(value string) error {
	fileNameRegex := "^[\\w]+.go$"
	lineNumberRegex := "^[\\d]+$"

	for _, f := range strings.Split(value, ",") {
		f := strings.Split(f, ":")
		if len(f) != 2 {
			return errors.New(
				fmt.Sprintf("Improperly formatted filter: %s, expected fname.go:line", f))
		}

		fname, lnumber := f[0], f[1]

		matched, err := regexp.Match(fileNameRegex, []byte(fname))
		if err != nil {
			return err
		}
		if !matched {
			return errors.New(
				fmt.Sprintf("Expected filename '%s' to match the regex '%s'", fname, fileNameRegex))
		}

		matched, err = regexp.Match(lineNumberRegex, []byte(lnumber))
		if err != nil {
			return err
		}
		if !matched {
			return errors.New(
				fmt.Sprintf("Expected line number '%s' to match the regex '%s'", lnumber, lineNumberRegex))
		}
		*l = append(*l, fmt.Sprintf("%s:%s", fname, lnumber))
	}

	return nil
}

func modeFromString(value string) (log.Mode, error) {
	var m log.Mode
	for _, mode := range strings.Split(value, "|") {
		switch mode {
		case "info":
			m |= log.InfoMode
		case "debug":
			m |= log.DebugMode
		case "warn":
			m |= log.WarnMode
		case "error":
			m |= log.ErrorMode
		case "disabled":
			m = log.DisabledMode
			break
		default:
			return m, errors.New(fmt.Sprintf("unrecognized mode: %v", m))
		}
	}
	return m, nil
}

func modeToString(m log.Mode) string {
	if m == log.DisabledMode {
		return "disabled"
	}

	var buf bytes.Buffer
	if (m & log.InfoMode) != log.DisabledMode {
		buf.WriteString("info|")
	}
	if (m & log.WarnMode) != log.DisabledMode {
		buf.WriteString("warn|")
	}
	if (m & log.ErrorMode) != log.DisabledMode {
		buf.WriteString("error|")
	}
	if (m & log.DebugMode) != log.DisabledMode {
		buf.WriteString("debug|")
	}
	return buf.String()[:len(buf.String())-1] // Chop off the last '|' symbol.
}
