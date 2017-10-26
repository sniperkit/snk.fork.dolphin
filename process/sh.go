package ps

import (
	"context"
	"syscall"

	sh "github.com/codeskyblue/go-sh"
	"we.com/dolphin/types"
)

var (
	ctrlScript = "/etc/dolphin/scripts/ctrl.sh"
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

func stop(ctx context.Context, args []string, env map[string]string) error {
	s := newCmd("stop", args, env)
	return execute(ctx, s)
}

func start(ctx context.Context, args []string, env map[string]string) error {
	s := newCmd("start", args, env)
	return execute(ctx, s)
}

func restart(ctx context.Context, args []string, env map[string]string) error {
	s := newCmd("restart", args, env)
	return execute(ctx, s)
}

// Execute an action
func Execute(ctx context.Context, action types.Command) types.CommandResult {
	ret := types.CommandResult{
		CommandID: action.ComandID,
		Success:   false,
		//Took      time.Duration `json:"took,omitempty"`
		//Output    []byte        `json:"output,omitempty"`
	}
	switch action.Type {
	case types.CMDStopInstance:
	case types.CMDStartInstance:
	case types.CMDRestartInstance:
	case types.CMDProbe:
	default:
	}

	return ret
}
