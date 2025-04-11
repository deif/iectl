package auth

import (
	"context"
	"net/http"
)

type contextKey struct{}

func NewContext(ctx context.Context, value *http.Client) context.Context {
	return context.WithValue(ctx, contextKey{}, value)
}

func FromContext(ctx context.Context) *http.Client {
	v, ok := ctx.Value(contextKey{}).(*http.Client)
	if !ok {
		return nil
	}
	return v
}
