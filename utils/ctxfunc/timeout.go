package ctxfunc

import (
	"context"
	"time"
)

type Func func(ctx context.Context)

func Timeout(d time.Duration, f Func) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	f(ctx)
}

func Timeout100ms(f Func) {
	Timeout(100*time.Millisecond, f)
}

func Timeout1s(f Func) {
	Timeout(1*time.Second, f)
}

func Timeout2s(f Func) {
	Timeout(2*time.Second, f)
}

func Timeout3s(f Func) {
	Timeout(3*time.Second, f)
}

func Timeout5s(f Func) {
	Timeout(5*time.Second, f)
}

func Timeout10s(f Func) {
	Timeout(10*time.Second, f)
}

func Timeout15s(f Func) {
	Timeout(15*time.Second, f)
}

func Timeout20s(f Func) {
	Timeout(20*time.Second, f)
}

func Timeout30s(f Func) {
	Timeout(30*time.Second, f)
}

func Timeout1m(f Func) {
	Timeout(60*time.Second, f)
}

func Timeout5m(f Func) {
	Timeout(5*time.Minute, f)
}

func Timeout10m(f Func) {
	Timeout(10*time.Minute, f)
}
