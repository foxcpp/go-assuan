package server

import (
	"io"
	"net"
	"os"

	"github.com/foxcpp/go-assuan/common"
)

// CommandHandler is an alias for command handler function type.
//
// state object is useful to store arbitrary data between transactions in
// single connection, it initialized from object returned by ProtoInfo.GetDefaultState.
type CommandHandler func(pipe io.ReadWriter, state interface{}, params string) *common.Error

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
}

// Serve function accepts incoming connection using specified protocol and initial state value.
func Serve(pipe io.ReadWriter, proto ProtoInfo) error {
	state := proto.GetDefaultState()
	if err := common.WriteLine(pipe, "OK", proto.Greeting); err != nil {
		return err
	}

	for {
		cmd, params, err := common.ReadLine(pipe)
		if err != nil {
			return err
		}

		if cmd == "BYE" {
			common.WriteLine(pipe, "OK", "")
			return nil
		}

		if cmd == "RESET" {
			if hndlr, prs := proto.Handlers["RESET"]; prs {
				err := hndlr(pipe, state, params)
				if err != nil {
					common.WriteError(pipe, *err)
				} else {
					common.WriteLine(pipe, "OK", "")
				}
			} else {
				// Default RESET handler: Reset context to null.
				state = nil
				common.WriteLine(pipe, "OK", "")
			}
		}

		if cmd == "HELP" {
			helpCmd(pipe, proto, params)
			continue
		}

		hndlr, prs := proto.Handlers[cmd]
		if !prs {
			common.WriteError(pipe, common.Error{
				common.ErrSrcAssuan, common.ErrAssUnknownCmd,
				"assuan", "unknown IPC command",
			})
			continue
		}

		if err := hndlr(pipe, state, params); err != nil {
			common.WriteError(pipe, *err)
		} else {
			common.WriteLine(pipe, "OK", "")
		}
	}
}

func helpCmd(pipe io.Writer, proto ProtoInfo, params string) {
	if len(params) != 0 {
		// Help requested for command.
		helpStrs, prs := proto.Help[params]
		if !prs {
			common.WriteError(pipe, common.Error{
				common.ErrSrcAssuan, common.ErrNotFound,
				"not found", "assuan",
			})
		} else {
			for _, helpStr := range helpStrs {
				common.WriteComment(pipe, helpStr)
			}
			common.WriteLine(pipe, "OK", "")
		}
	} else {
		// Just HELP, print commands.
		for k := range proto.Handlers {
			common.WriteComment(pipe, k)
		}
		common.WriteLine(pipe, "OK", "")
	}
}

// ServeStdin is same as Serve but uses stdin and stdout as communication channel.
func ServeStdin(proto ProtoInfo) error {
	return Serve(common.ReadWriter{os.Stdout, os.Stdin}, proto)
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
			return err
		}
		go func() {
			defer conn.Close()
			Serve(conn, proto)
		}()
	}
}
