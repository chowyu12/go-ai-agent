package tool

import "context"

type Tool interface {
	Name() string
	Description() string
	Call(ctx context.Context, input string) (string, error)
}

type BuiltinHandler func(ctx context.Context, args string) (string, error)
