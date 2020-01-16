package client

import (
	"encoding"
	"errors"
	"io"
	"os/exec"

	"github.com/foxcpp/go-assuan/common"
)

// Session struct is a wrapper which represents an alive connection between
// client and server.
//
// In Assuan protocol roles of peers after handshake is not same, for this
// reason there is no generic Session object that will work for both client and
// server. In particular, client.Session (the struct you are looking at)
// represents client side of connection.
type Session struct {
	Pipe common.Pipe
}

// Init initiates session using passed Reader/Writer.
func Init(stream io.ReadWriter) (*Session, error) {
	Logger.Println("Starting session...")
	ses := &Session{common.New(stream)}

	// Take server's OK from pipe.
	_, _, err := ses.Pipe.ReadLine()
	if err != nil {
		Logger.Println("... I/O error:", err)
		return nil, err
	}

	return ses, nil
}

// InitCmd initiates session using command's stdin and stdout as a I/O channel.
// cmd.Start() will be done by this function and should not be done before.
//
// Warning: It's caller's responsibility to close pipes set in exec.Cmd
// object (cmd.Stdin, cmd.Stdout).
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
		Logger.Println("Failed to start command ("+cmd.Path+"):", err)
		return nil, err
	}

	ses, err := Init(common.ReadWriter{Reader: stdout, Writer: stdin})
	if err != nil {
		return nil, err
	}
	return ses, nil
}

// Close sends BYE and closes underlying pipe.
func (ses *Session) Close() error {
	Logger.Println("Closing session (sending BYE)...")
	if err := ses.Pipe.WriteLine("BYE", ""); err != nil {
		Logger.Println("... I/O error:", err)
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
	Logger.Println("Resetting session...")
	_, err := ses.SimpleCmd("RESET", "")
	return err
}

// SimpleCmd sends command with specified parameters and reads data sent by server if any.
func (ses *Session) SimpleCmd(cmd string, params string) (data []byte, err error) {
	Logger.Println("Sending command:", cmd, params)
	err = ses.Pipe.WriteLine(cmd, params)
	if err != nil {
		Logger.Println("... I/O error:", err)
		return []byte{}, err
	}

	for {
		scmd, sparams, err := ses.Pipe.ReadLine()
		if err != nil {
			Logger.Println("... I/O error:", err)
			return []byte{}, err
		}

		if scmd == "OK" {
			if data != nil && data[len(data)-1] == '\n' {
				data = data[:len(data)-1]
			}
			return data, nil
		}
		if scmd == "ERR" {
			Logger.Println("... Received ERR: ", sparams)
			return []byte{}, common.DecodeErrCmd(sparams)
		}
		if scmd == "D" {
			data = append(data, []byte(sparams)...)
			data = append(data, "\n"...)
		}
		if scmd == "S" {
			data = append(data, []byte(sparams)...)
			data = append(data, "\n"...)
		}
	}
}

// Transact sends command with specified params and uses byte arrays in data
// argument to answer server's inquiries. Values in data can be either []byte
// or pointer to implementer of io.Reader or encoding.TextMarhshaller.
func (ses *Session) Transact(cmd string, params string, data map[string]interface{}) (rdata []byte, err error) {
	Logger.Println("Initiating transaction:", cmd, params)
	err = ses.Pipe.WriteLine(cmd, params)
	if err != nil {
		return nil, err
	}

	for {
		scmd, sparams, err := ses.Pipe.ReadLine()
		if err != nil {
			return nil, err
		}

		if scmd == "INQUIRE" {
			inquireResp, prs := data[sparams]
			if !prs {
				Logger.Println("... unknown request:", sparams)
				if err := ses.Pipe.WriteLine("CAN", ""); err != nil {
					return nil, err
				}

				// We asked for FOO but we don't have FOO.
				return nil, errors.New("missing data with keyword " + sparams)
			}

			switch inquireResp.(type) {
			case []byte:
				if err := ses.Pipe.WriteData(inquireResp.([]byte)); err != nil {
					Logger.Println("... I/O error:", err)
					return nil, err
				}
			case io.Reader:
				if err := ses.Pipe.WriteDataReader(inquireResp.(io.Reader)); err != nil {
					Logger.Println("... I/O error:", err)
					return nil, err
				}
			case encoding.TextMarshaler:
				marhshalled, err := inquireResp.(encoding.TextMarshaler).MarshalText()
				if err != nil {
					return nil, err
				}
				if err := ses.Pipe.WriteData(marhshalled); err != nil {
					Logger.Println("... I/O error:", err)
					return nil, err
				}
			default:
				return nil, errors.New("invalid type in data map value")
			}

			if err := ses.Pipe.WriteLine("END", ""); err != nil {
				Logger.Println("... I/O error:", err)
				return nil, err
			}
		}

		// Same as SimpleCmd.
		if scmd == "OK" {
			return rdata, nil
		}
		if scmd == "ERR" {
			Logger.Println("... Received ERR: ", sparams)
			return []byte{}, common.DecodeErrCmd(sparams)
		}
		if scmd == "D" {
			Logger.Println("... Received data chunk")
			rdata = append(rdata, []byte(sparams)...)
		}
	}
}

// Option sets options for connections.
func (ses *Session) Option(name string, value string) error {
	Logger.Println("Setting option", name, "to", value+"...")
	_, err := ses.SimpleCmd("OPTION", name+" = "+value)
	return err
}
