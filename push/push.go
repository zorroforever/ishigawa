package push

import "golang.org/x/net/context"

type Service interface {
	Push(ctx context.Context, udid string) (string, error)
}
