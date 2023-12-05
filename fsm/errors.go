package fsm

import (
	"errors"
	"fmt"
)

type StatusFsmError struct {
	Msg string
}

type TransFsmError struct {
	Dst    string
	Msg    string
	Detail error
}

func (e TransFsmError) Error() string {
	return fmt.Sprintf("dst: %s\nmsg: %s\ndetail: %v", e.Dst, e.Msg, e.Detail)
}

var (
	// FSM error
	ErrStateNotAvailable        = errors.New("FSM state is not available")
	ErrFSMIsRunning             = errors.New("FSM is running")
	ErrFSMRegisterError         = errors.New("FSM register error")
	ErrFSMTransitionNotRegister = errors.New("FSM event transition executor is needed")
	ErrFSMCallbackNewNeeded     = errors.New("FSM NEW callback need to register for blocking the state when mode is ModeSingle")
	// FSM cluster error
	ErrClusterWrong = errors.New("some thing unexpect")
)
