package common

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

const (
	// MaxLineLen is a maximum length of line in Assuan protocol, including
	// space after command and LF.
	MaxLineLen = 1000
)

// Context is a base type for Assuan I/O. It's like a net.Conn.
// You should use client.Session or server.Session depending on what you need.
// This structure is only a thin wrapper for I/O functions.
type Context struct {
	Pipe    io.ReadWriteCloser
	scanner *bufio.Scanner
}

// NewContext creates new context using specified io.ReadWriteCloser.
//
// Scanner's buffer is restricted to MaxLineLen to enforce line length
// limit for incoming commands.
func NewContext(pipe io.ReadWriteCloser) *Context {
	ctx := new(Context)
	ctx.Pipe = pipe
	ctx.scanner = bufio.NewScanner(ctx.Pipe)
	ctx.scanner.Buffer(make([]byte, MaxLineLen), MaxLineLen)
	return ctx
}

// Close closes context's underlying pipe.
func (ctx *Context) Close() error {
	return ctx.Pipe.Close()
}

// ReadLine reads raw request/response in following format: command <parameters>
//
// Empty lines and lines starting with # are ignored as specified by protocol.
// Additinally, status information is silently discarded for now.
func (ctx *Context) ReadLine() (cmd string, params string, err error) {
	var line string
	for {
		if ok := ctx.scanner.Scan(); !ok {
			return "", "", ctx.scanner.Err()
		}
		line = ctx.scanner.Text()

		// We got something that looks like a message. Let's parse it.
		if !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "S ") && len(strings.TrimSpace(line)) != 0 {
			break
		}
	}

	// Part before first whitespace is a command. Everything after first whitespace is paramters.
	parts := strings.SplitN(line, " ", 2)

	// If there is no parameters... (huh!?)
	if len(parts) == 1 {
		return strings.ToUpper(parts[0]), "", nil
	}

	params, err = unescapeParameters(parts[1])
	if err != nil {
		return "", "", err
	}

	// Command is "normalized" to upper case since peer can send
	// commands in any case.
	return strings.ToUpper(parts[0]), params, nil
}

// WriteLine writes request/response to underlying pipe.
// Contents of params is escaped according to requirements of Assuan protocol.
func (ctx *Context) WriteLine(cmd string, params string) error {
	if len(cmd)+len(params)+2 > MaxLineLen {
		// 2 is for whitespace after command and LF
		return errors.New("too long command or parameters")
	}

	line := []byte(strings.ToUpper(cmd) + " " + escapeParameters(params) + "\n")
	_, err := ctx.Pipe.Write(line)
	return err
}

func min(a, b int) int {
	if a <= b {
		return a
	}
	return b
}

// WriteData sends passed byte slice using one or more D commands.
// Note: Error may occur even after some data is written so it's better
// to just CAN transaction after WriteData error.
func (ctx *Context) WriteData(input []byte) error {
	encoded := []byte(escapeParameters(string(input)))
	chunkLen := MaxLineLen - 3 // 3 is for 'D ' and line feed.
	for i := 0; i < len(encoded); i += chunkLen {
		chunk := encoded[i:min(i+chunkLen, len(encoded))]
		chunk = append([]byte{'D', ' '}, chunk...)
		chunk = append(chunk, '\n')

		if _, err := ctx.Pipe.Write(chunk); err != nil {
			return err
		}
	}
	return nil
}

// WriteComment is special case of WriteLine. "Command" is # and text is parameter.
func (ctx *Context) WriteComment(text string) error {
	return ctx.WriteLine("#", text)
}
