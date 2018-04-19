package client

import (
	"bufio"
	"errors"
	"io"
	"os/exec"

	"github.com/foxcpp/go-assuan/common"
)

// Session struct is a wrapper which represents an alive connection between
// client and server.
//
// In Assuan protocol roles of peers after handleshake is not same, for this
// reason there is no generic Session object that will work for both client and
// server. In pracicular, client.Session (the struct you are looking at)
// represents client side of connection.
type Session struct {
	Pipe    io.ReadWriteCloser
	Scanner *bufio.Scanner
}

type readWriteCloser struct {
	io.ReadCloser
	io.WriteCloser
}

func (rwc readWriteCloser) Close() error {
	if err := rwc.ReadCloser.Close(); err != nil {
		return err
	}
	if err := rwc.WriteCloser.Close(); err != nil {
		return err
	}
	return nil
}

// Implements no-op Close() function in additional to holding reference to
// Reader and Writer.
type nopCloser struct {
	io.ReadWriter
}

func (clsr nopCloser) Close() error {
	return nil
}

// InitNopClose initiates session using passed Reader/Writer and NOP closer.
func InitNopClose(pipe io.ReadWriter) *Session {
	ses := &Session{nopCloser{pipe}, bufio.NewScanner(pipe)}
	ses.Scanner.Buffer(make([]byte, common.MaxLineLen), common.MaxLineLen)

	// Take server's OK from pipe.
	common.ReadLine(ses.Scanner)

	return ses
}

// Init initiates session using passed Reader/Writer.
func Init(pipe io.ReadWriteCloser) *Session {
	ses := &Session{pipe, bufio.NewScanner(pipe)}
	ses.Scanner.Buffer(make([]byte, common.MaxLineLen), common.MaxLineLen)

	// Take server's OK from pipe.
	common.ReadLine(ses.Scanner)

	return ses
}

// InitCmd initiates session using command's stdin and stdout as a I/O channel.
// cmd.Start() will be done by this function and should not be done before.
func InitCmd(cmd *exec.Cmd) (*Session, error) {
	// Errors generally should not happen here but let's be pedantic because we are library.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return Init(readWriteCloser{stdout, stdin}), nil
}

// Close sends BYE and closes underlying pipe.
func (ses *Session) Close() error {
	if err := common.WriteLine(ses.Pipe, "BYE", ""); err != nil {
		return err
	}
	// Server should respond with "OK" , but we don't care.
	return ses.Pipe.Close()
}

// Reset sends RESET command.
// According to Assuan documentation: Reset the connection but not any existing
// authentication. The server should release all resources associated with the
// connection.
func (ses *Session) Reset() error {
	if err := common.WriteLine(ses.Pipe, "RESET", ""); err != nil {
		return err
	}
	// Take server's OK from pipe.
	ok, params, err := common.ReadLine(ses.Scanner)
	if err != nil {
		return err
	}
	if ok == "ERR" {
		return common.DecodeErrCmd(params)
	}
	if ok != "OK" {
		return errors.New("not 'ok' response")
	}
	return nil
}

// SimpleCmd sends command with specified parameters and reads data sent by server if any.
func (ses *Session) SimpleCmd(cmd string, params string) (data []byte, err error) {
	err = common.WriteLine(ses.Pipe, cmd, params)
	if err != nil {
		return []byte{}, err
	}

	for {
		scmd, sparams, err := common.ReadLine(ses.Scanner)
		if err != nil {
			return []byte{}, err
		}

		if scmd == "OK" {
			return data, nil
		}
		if scmd == "ERR" {
			return []byte{}, common.DecodeErrCmd(sparams)
		}
		if scmd == "D" {
			data = append(data, []byte(sparams)...)
		}
	}
}

// Transact sends command with specified params and uses byte arrays in data
// argument to answer server's inquiries.
func (ses *Session) Transact(cmd string, params string, data map[string][]byte) (rdata []byte, err error) {
	err = common.WriteLine(ses.Pipe, cmd, params)
	if err != nil {
		return []byte{}, err
	}

	for {
		scmd, sparams, err := common.ReadLine(ses.Scanner)
		if err != nil {
			return []byte{}, err
		}

		if scmd == "INQUIRE" {
			inquireResp, prs := data[sparams]
			if !prs {
				common.WriteLine(ses.Pipe, "CAN", "")
				// TODO: Which error (write err or missing data) is more
				// important because we can't return both?

				// We asked for FOO but we don't have FOO.
				return []byte{}, errors.New("missing data with keyword " + sparams)
			}

			if err := common.WriteData(ses.Pipe, inquireResp); err != nil {
				return []byte{}, err
			}
			if err := common.WriteLine(ses.Pipe, "END", ""); err != nil {
				return []byte{}, err
			}
		}

		// Same as SimpleCmd.
		if scmd == "OK" {
			return rdata, nil
		}
		if scmd == "ERR" {
			return []byte{}, common.DecodeErrCmd(sparams)
		}
		if scmd == "D" {
			rdata = append(rdata, []byte(sparams)...)
		}
	}
}

// Option sets options for connections.
func (ses *Session) Option(name string, value string) error {
	err := common.WriteLine(ses.Pipe, "OPTION", name+" = "+value)
	if err != nil {
		return err
	}

	cmd, sparams, err := common.ReadLine(ses.Scanner)
	if err != nil {
		return err
	}
	if cmd == "ERR" {
		return common.DecodeErrCmd(sparams)
	}

	return nil
}
