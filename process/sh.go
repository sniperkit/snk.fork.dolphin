/*
Sniperkit-Bot
- Status: analyzed
*/

package ps

import (
	"context"
	"syscall"
	"time"

	sh "github.com/codeskyblue/go-sh"
	"github.com/pkg/errors"
	"we.com/dolphin/types"
)

var (
	ctrlScript = "/etc/telegraf/scripts/ctrl.sh"
)

func goo(f func() error) chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- f()
	}()
	return ch
}

func execute(ctx context.Context, s *sh.Session) error {
	if err := s.Start(); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		s.Kill(syscall.SIGKILL)
		return ctx.Err()
	case err := <-goo(s.Wait):
		return err
	}
}

func newCmd(cmd string, args []string, env map[string]string) *sh.Session {
	cargs := make([]interface{}, len(args)+1)
	cargs[0] = cmd
	for k, v := range args {
		cargs[k+1] = v
	}
	s := sh.Command(ctrlScript, cargs...)
	s.Env = env
	s.ShowCMD = true
	return s
}

func stop(ctx context.Context, args []string, env map[string]string) ([]byte, error) {
	s := newCmd("stop", args, env)
	err := execute(ctx, s)
	if err != nil {
		return s.Output()
	}
	o, _ := s.Output()
	return o, err
}

func start(ctx context.Context, args []string, env map[string]string) ([]byte, error) {
	s := newCmd("start", args, env)
	err := execute(ctx, s)
	if err != nil {
		return s.Output()
	}
	o, _ := s.Output()
	return o, err
}

func restart(ctx context.Context, args []string, env map[string]string) ([]byte, error) {
	s := newCmd("restart", args, env)
	err := execute(ctx, s)
	if err != nil {
		return s.Output()
	}
	o, _ := s.Output()
	return o, err
}

// Execute an action
func Execute(ctx context.Context, action types.Command) types.CommandResult {
	ret := types.CommandResult{
		CommandID: action.ComandID,
		Success:   false,
		//Took      time.Duration `json:"took,omitempty"`
		//Output    []byte        `json:"output,omitempty"`
	}

	s := time.Now()
	var sesstion *sh.Session

	switch action.Type {
	case types.CMDStopInstance:
		args, ok := action.Args.([]string)
		if !ok {
			ret.Err = errors.New("invalid args for stop instance command, expect args be []string")
		} else {
			sesstion = newCmd("stop", args, action.Envs)
		}

	case types.CMDStartInstance:
		args, ok := action.Args.([]string)
		if !ok {
			ret.Err = errors.New("invalid args for start instance command, expect args be []string")
		} else {
			sesstion = newCmd("start", args, action.Envs)
		}

	case types.CMDRestartInstance:
		args, ok := action.Args.([]string)
		if !ok {
			ret.Err = errors.New("invalid args for restart instance command, expect args be []string")
		} else {
			sesstion = newCmd("restart", args, action.Envs)
		}
	case types.CMDProbe:
		ret.Err = errors.Errorf("unsupport command: probe")
	default:
		ret.Err = errors.Errorf("unsupport command: %v", action.Type)
	}

	if sesstion == nil {
		return ret
	}

	if err := execute(ctx, sesstion); err != nil {
		ret.Err = err
	}

	e := time.Now()
	ret.Took = e.Sub(s)
	o, err := sesstion.Output()
	ret.Output = o
	if ret.Err == nil {
		ret.Err = err
	}

	return ret
}

func startCmd(ver types.DeployVer, key types.DeployKey) *types.Command {
	envMap := map[string]string{
		envDeployKey:  string(key),
		envVersion:    string(ver),
		envInstanceID: string(types.NewInstanceID()),
	}
	action := types.Command{
		Type:           types.CMDStartInstance,
		Args:           []string{string(key), string(ver)},
		ExecuteTimeout: time.Minute,
		Needout:        false,
		Envs:           envMap,
		OutKeep:        6 * time.Hour,
	}

	return &action
}

func restartCmd(v *types.Instance) *types.Command {
	args := v.StopCmdArgs()
	action := types.Command{
		Type:           types.CMDRestartInstance,
		Args:           args[:],
		ExecuteTimeout: time.Minute,
		Needout:        false,
		OutKeep:        6 * time.Hour,
	}

	return &action
}

func stopCmd(v *types.Instance) *types.Command {
	args := v.StopCmdArgs()
	action := types.Command{
		Type:           types.CMDStopInstance,
		Args:           args[:],
		ExecuteTimeout: time.Minute,
		Needout:        false,
		OutKeep:        6 * time.Hour,
	}

	return &action
}
