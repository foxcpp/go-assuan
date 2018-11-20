package pinentry

import (
	"strconv"
	"strings"
	"time"

	"github.com/foxcpp/go-assuan/common"
	"github.com/foxcpp/go-assuan/server"
)

type Callbacks struct {
	GetPIN  func(Settings) (string, *common.Error)
	Confirm func(Settings) (bool, *common.Error)
	Msg     func(Settings) *common.Error
}

func setDesc(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).Desc = params
	return nil
}
func setPrompt(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).Prompt = params
	return nil
}
func setRepeat(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).RepeatPrompt = params
	return nil
}
func setRepeatError(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).RepeatError = params
	return nil
}
func setError(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).Error = params
	return nil
}
func setOk(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).OkBtn = params
	return nil
}
func setNotOk(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).NotOkBtn = params
	return nil
}
func setCancel(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).CancelBtn = params
	return nil
}
func setQualityBar(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).QualityBar = params
	return nil
}
func setTitle(_ *common.Pipe, state interface{}, params string) error {
	state.(*Settings).Title = params
	return nil
}
func setTimeout(_ *common.Pipe, state interface{}, params string) error {
	i, err := strconv.Atoi(params)
	if err != nil {
		return &common.Error{
			Src: common.ErrSrcPinentry, Code: common.ErrAssInvValue,
			SrcName: "pinentry", Message: "invalid timeout value",
		}
	}
	state.(*Settings).Timeout = time.Duration(i)
	return nil
}
func setOpt(state interface{}, key string, val string) error {
	opts := state.(*Settings)

	if key == "no-grab" {
		opts.Opts.Grab = false
		return nil
	}
	if key == "grab" {
		opts.Opts.Grab = true
		return nil
	}
	if key == "ttytype" {
		opts.Opts.TTYType = val
		return nil
	}
	if key == "ttyname" {
		opts.Opts.TTYName = val
		return nil
	}
	if key == "ttyalert" {
		opts.Opts.TTYAlert = val
		return nil
	}
	if key == "lc-ctype" {
		opts.Opts.LCCtype = val
		return nil
	}
	if key == "lc-messages" {
		opts.Opts.LCMessages = val
		return nil
	}
	if key == "owner" {
		opts.Opts.Owner = val
		return nil
	}
	if key == "touch-file" {
		opts.Opts.TouchFile = val
		return nil
	}
	if key == "parent-wid" {
		opts.Opts.ParentWID = val
		return nil
	}
	if key == "invisible-char" {
		opts.Opts.InvisibleChar = val
		return nil
	}
	if key == "allow-external-password-cache" {
		opts.Opts.AllowExtPasswdCache = true
		return nil
	}

	if strings.HasPrefix(key, "default-") {
		return nil
	}

	return &common.Error{
		Src: common.ErrSrcPinentry, Code: common.ErrUnknownOption,
		SrcName: "pinentry", Message: "unknown option: " + key,
	}
}

func resetState(_ *common.Pipe, state interface{}, _ string) error {
	*(state.(*Settings)) = Settings{}
	return nil
}

var ProtoInfo = server.ProtoInfo{
	Greeting: "go-assuan pinentry",
	Handlers: map[string]server.CommandHandler{
		"SETDESC":        setDesc,
		"SETPROMPT":      setPrompt,
		"SETREPEAT":      setRepeat,
		"SETREPEATERROR": setRepeatError,
		"SETERROR":       setError,
		"SETOK":          setOk,
		"SETNOTOK":       setNotOk,
		"SETCANCEL":      setCancel,
		"SETQUALITYBAR":  setQualityBar,
		"SETTITLE":       setTitle,
		"SETTIMEOUT":     setTimeout,
		"RESET":          resetState,
	},
	Help: map[string][]string{}, // TODO
	GetDefaultState: func() interface{} {
		return Settings{}
	},
	SetOption: setOpt,
}

func Serve(callbacks Callbacks, customGreeting string) error {
	info := ProtoInfo

	if len(customGreeting) != 0 {
		info.Greeting = customGreeting
	}

	info.Handlers["GETPIN"] = func(pipe *common.Pipe, state interface{}, _ string) error {
		if callbacks.GetPIN == nil {
			Logger.Println("GETPIN requested but not supported")
			return &common.Error{
				Src: common.ErrSrcPinentry, Code: common.ErrNotImplemented,
				SrcName: "pinentry", Message: "GETPIN op is not supported",
			}
		}

		pass, err := callbacks.GetPIN(*state.(*Settings))
		if err != nil {
			return err
		}

		if err := pipe.WriteData([]byte(pass)); err != nil {
			return nil
		}
		return nil
	}
	info.Handlers["CONFIRM"] = func(pipe *common.Pipe, state interface{}, _ string) error {
		if callbacks.Confirm == nil {
			Logger.Println("CONFIRM requested but not supported")
			return &common.Error{
				Src: common.ErrSrcPinentry, Code: common.ErrNotImplemented,
				SrcName: "pinentry", Message: "CONFIRM op is not supported",
			}
		}

		v, err := callbacks.Confirm(*state.(*Settings))
		if err != nil {
			return err
		}

		if !v {
			return &common.Error{
				Src: common.ErrSrcPinentry, Code: common.ErrCanceled,
				SrcName: "pinentry", Message: "operation canceled",
			}
		}
		return nil
	}
	info.Handlers["MESSAGE"] = func(pipe *common.Pipe, state interface{}, _ string) error {
		if callbacks.Msg == nil {
			Logger.Println("MESSAGE requested but not supported")
			return &common.Error{
				Src: common.ErrSrcPinentry, Code: common.ErrNotImplemented,
				SrcName: "pinentry", Message: "MESSAGE op is not supported",
			}
		}

		return callbacks.Msg(*state.(*Settings))
	}

	err := server.ServeStdin(info)
	return err
}
