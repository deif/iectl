package target

import (
	"context"
)

type contextKey struct{}

func NewContext(ctx context.Context, value Collection) context.Context {
	return context.WithValue(ctx, contextKey{}, value)
}

func FromContext(ctx context.Context) Collection {
	v, ok := ctx.Value(contextKey{}).(Collection)
	if !ok {
		return nil
	}
	return v
}
