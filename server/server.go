package server

import (
	"io"
	"net"
	"os"
	"regexp"

	"github.com/foxcpp/go-assuan/common"
)

// CommandHandler is an alias for command handler function type.
//
// state object is useful to store arbitrary data between transactions in
// single connection, it initialized from object returned by ProtoInfo.GetDefaultState.
//
// Handler can report two kinds of errors: Protocol errors and "other" errors.
// First will be just sent to client, second will terminate session (you want to report I/O errors this way).
type CommandHandler func(pipe *common.Pipe, state interface{}, params string) (*common.Error, error)

// ProtoInfo describes how to handle commands sent from client on server.
// Usually there is only one instance of this structure per protocol (i.e. in global variable).
type ProtoInfo struct {
	// Sent together with first OK.
	Greeting string
	// Key is command name (in uppercase), handler is called when specific command is received.
	Handlers map[string]CommandHandler
	// Help strings for commands, splitten by \n.
	Help map[string][]string
	// Function that should return newly allocated state object for protocol.
	GetDefaultState func() interface{}
	// Function that should set option passed via OPTION command or return an error.
	SetOption func(state interface{}, key string, val string) *common.Error
}

var optRegexp = regexp.MustCompile(`^([\d\w\-]+)(?:[ =](.*))?$`)

func splitOption(params string) (key string, val string, err *common.Error) {
	groups := optRegexp.FindStringSubmatch(params)
	if groups == nil {
		return "", "", &common.Error{
			Src: common.ErrSrcAssuan, Code: common.ErrAssInvValue,
			SrcName: "assuan", Message: "invalid OPTION syntax",
		}
	}

	return groups[1], groups[2], nil
}

// Serve function accepts incoming connection using specified protocol and initial state value.
func Serve(stream io.ReadWriter, proto ProtoInfo) error {
	Logger.Println("Accepted session")
	pipe := common.New(stream)

	state := proto.GetDefaultState()
	if err := pipe.WriteLine("OK", proto.Greeting); err != nil {
		Logger.Println("I/O error, dropping session:", err)
		return err
	}

	for {
		cmd, params, err := pipe.ReadLine()
		if err != nil {
			Logger.Println("I/O error, dropping session:", err)
			return err
		}

		switch cmd {
		case "BYE":
			if err := pipe.WriteLine("OK", ""); err != nil {
				Logger.Println("... IO error, dropping session:", err)
				return err
			}
			Logger.Println("Session finished")
		case "NOP":
			if err := pipe.WriteLine("OK", ""); err != nil {
				Logger.Println("... IO error, dropping session:", err)
				return err
			}
		case "RESET":
			if err := resetCmd(&pipe, &state, proto); err != nil {
				Logger.Println("... IO error, dropping session:", err)
				return err
			}
		case "OPTION":
			if err := optionCmd(&pipe, state, proto, params); err != nil {
				Logger.Println("... IO error, dropping session:", err)
				return err
			}
		case "HELP":
			if err := helpCmd(&pipe, proto, params); err != nil {
				Logger.Println("... IO error, dropping session:", err)
				return err
			}
		default:
			Logger.Println("Protocol command received:", cmd)
			hndlr, prs := proto.Handlers[cmd]
			if !prs {
				Logger.Println("... unknown command:", cmd)
				if err := pipe.WriteError(common.Error{
					Src: common.ErrSrcAssuan, Code: common.ErrAssUnknownCmd,
					SrcName: "assuan", Message: "unknown IPC command",
				}); err != nil {
					Logger.Println("... IO error, dropping session:", err)
					return err
				}
				continue
			}

			perr, err := hndlr(&pipe, state, params)
			if err != nil {
				Logger.Println("... handler error:", err)
				return err
			}
			if perr != nil {
				Logger.Println("... handler error:", perr)
				if err := pipe.WriteError(*perr); err != nil {
					Logger.Println("... IO error, dropping session:", err)
					return err
				}
			} else {
				if err := pipe.WriteLine("OK", ""); err != nil {
					Logger.Println("... IO error, dropping session:", err)
					return err
				}
			}
		}
	}
}

func helpCmd(pipe *common.Pipe, proto ProtoInfo, params string) error {
	Logger.Println("Help request")

	if len(params) != 0 {
		// Help requested for command.
		helpStrs, prs := proto.Help[params]
		if !prs {
			Logger.Println("Help requested for unknown command:", params)
			if err := pipe.WriteError(common.Error{
				Src: common.ErrSrcAssuan, Code: common.ErrNotFound,
				SrcName: "assuan", Message: "not found",
			}); err != nil {
				return err
			}
		} else {
			for _, helpStr := range helpStrs {
				if err := pipe.WriteComment(helpStr); err != nil {
					return err
				}
			}
			if err := pipe.WriteLine("OK", ""); err != nil {
				return err
			}
		}
	} else {
		// Just HELP, print commands.
		for _, cmd := range [8]string{"NOP", "OPTION", "CANCEL", "BYE", "RESET", "END", "HELP"} {
			if err := pipe.WriteComment(cmd); err != nil {
				return err
			}
		}
		for k := range proto.Handlers {
			if err := pipe.WriteComment(k); err != nil {
				return err
			}
		}
		if err := pipe.WriteLine("OK", ""); err != nil {
			return err
		}
	}
	return nil
}

func resetCmd(pipe *common.Pipe, state *interface{}, proto ProtoInfo) error {
	Logger.Println("Session reset")
	if hndlr, prs := proto.Handlers["RESET"]; prs {
		perr, err := hndlr(pipe, *state, "")
		if err != nil {
			return err
		}
		if perr != nil {
			if err := pipe.WriteError(*perr); err != nil {
				return err
			}
		} else {
			if err := pipe.WriteLine("OK", ""); err != nil {
				return err
			}
		}
	} else {
		// Default RESET handler: Reset context to null.
		*state = nil
		if err := pipe.WriteLine("OK", ""); err != nil {
			return err
		}
	}
	return nil
}

func optionCmd(pipe *common.Pipe, state interface{}, proto ProtoInfo, params string) error {
	Logger.Println("Option set request:", params)
	if proto.SetOption == nil {
		Logger.Println("... no options supported in this protocol")
		if err := pipe.WriteError(common.Error{
			Src: common.ErrSrcAssuan, Code: common.ErrNotImplemented,
			SrcName: "assuan", Message: "not implemented",
		}); err != nil {
			return err
		}
		return nil
	}
	key, value, err := splitOption(params)
	if err != nil {
		Logger.Println("... malformed request: ", err)
		if err := pipe.WriteError(*err); err != nil {
			return err
		}
		return nil
	}
	err = proto.SetOption(state, key, value)
	if err != nil {
		Logger.Println("... invalid option:", err)
		if err := pipe.WriteError(*err); err != nil {
			return err
		}
	}
	if err := pipe.WriteLine("OK", ""); err != nil {
		return err
	}
	return nil
}

// ServeStdin is same as Serve but uses stdin and stdout as communication channel.
func ServeStdin(proto ProtoInfo) error {
	return Serve(common.ReadWriter{Reader: os.Stdin, Writer: os.Stdout}, proto)
}

// Listener is a minimal interface implemented by net.UnixListener and net.TCPListener.
type Listener interface {
	Accept() (net.Conn, error)
}

// ServeNet is same as Server but accepts connections (net.Conn) using passed
// listener and launches goroutine to serve each.
// This function will return if Accept() fails.
func ServeNet(listener Listener, proto ProtoInfo) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			Logger.Println("Listener fail:", err)
			continue
		}
		Logger.Println("Received remote connection on", conn.LocalAddr(), "from", conn.RemoteAddr())
		go func() {
			defer conn.Close()
			if err := Serve(conn, proto); err != nil {
				Logger.Println("Serve fail:", err)
			}
		}()
	}
	return nil
}
