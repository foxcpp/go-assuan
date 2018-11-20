package server_test

import (
	"bufio"
	"fmt"
	"os"

	"github.com/foxcpp/go-assuan/common"
	"github.com/foxcpp/go-assuan/server"
)

type State struct {
	desc string
}

func setdesc(_ *common.Pipe, state interface{}, params string) *common.Error {
	state.(*State).desc = params
	return nil
}

func getpin(pipe *common.Pipe, state interface{}, _ string) *common.Error {
	s := bufio.NewScanner(os.Stdout)
	fmt.Println(state.(*State).desc)
	fmt.Print("Enter PIN: ")
	if ok := s.Scan(); !ok {
		return &common.Error{
			Src: common.ErrSrcUnknown, Code: common.ErrGeneral,
			SrcName: "system", Message: "I/O error",
		}
	}
	pipe.WriteData(s.Bytes())
	return nil
}

func ExampleProtoInfo() {
	pinentry := server.ProtoInfo{
		Greeting: "Pleased to meet you",
		Handlers: map[string]server.CommandHandler{
			"SETDESC": server.CommandHandler(setdesc),
			"GETPIN":  server.CommandHandler(getpin),
		},
		Help: map[string][]string{
			"SETDESC": {
				"Set request description",
			},
			"GETPIN": {
				"Read string from TTY",
			},
		},
		GetDefaultState: func() interface{} {
			return &State{"default desc"}
		},
	}
	server.ServeStdin(pinentry)
}
