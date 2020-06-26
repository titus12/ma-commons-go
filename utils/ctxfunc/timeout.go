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
