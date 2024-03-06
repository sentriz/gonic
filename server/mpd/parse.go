package mpd

import (
	"bufio"
	"strings"
)

type lineReader struct {
	inner *bufio.Reader
	idx   uint
}

func newLineReader(rd *bufio.Reader) *lineReader {
	return &lineReader{rd, 0}
}

func (lr *lineReader) Next() (uint, string, error) {
	line, err := lr.inner.ReadString('\n')
	if err != nil {
		return 0, "", err
	}

	lr.idx += 1
	line = strings.TrimSuffix(line, "\n")

	return lr.idx, line, nil
}

// argParser splits a line into a sequence of arguments.
// The first argument is mandatory and is the command.
type argParser struct {
	line string
	buff []rune
	idx  uint // argument index
}

func newArgParser(line string) *argParser {
	return &argParser{line, []rune(line), 0}
}

func (p *argParser) pos() int {
	return len(p.line) - len(p.buff)
}

func (p *argParser) Next() (uint, string, bool) {
	const (
		linearSpace = " \t"

		stateSpace = iota
		stateArg
		stateQuote
	)

	if len(p.buff) == 0 {
		return p.idx, "", false
	}

	res := make([]rune, 0, len(p.buff))

	i := 0
loop:
	for state := stateSpace; i < len(p.buff); i++ {
		r := p.buff[i]
		isSpace := strings.IndexRune(linearSpace, r) != -1

		if state == stateSpace {
			if isSpace {
				continue
			}

			state = stateArg
		}

		switch {
		case state == stateArg && isSpace:
			break loop // space -> end of argument

		case state == stateArg && r == '"':
			state = stateQuote // quote begin
			continue

		case state == stateQuote && r == '\\':
			i++ // skip the backslash

		case state == stateQuote && r == '"':
			state = stateArg // quote end
			continue
		}

		res = append(res, p.buff[i])
	}

	p.idx += 1
	p.buff = p.buff[i:]

	return p.idx, string(res), true
}
