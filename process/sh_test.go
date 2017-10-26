package ps

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"
)

func Test_execute(t *testing.T) {
	ctrlScript = "ping"
	for {
		s := newCmd("localhost", []string{"-n", "6"}, nil)
		ctx := context.Background()
		ctx, cf := context.WithCancel(ctx)
		go func() {
			err := execute(ctx, s)
			fmt.Printf("err: %v\n", err)
		}()
		time.Sleep(2 * time.Second)
		cf()
		fmt.Printf("num of goroutins: %v", runtime.NumGoroutine())
	}
}
