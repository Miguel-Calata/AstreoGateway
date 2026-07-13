package admin

import "context"

func withAdminID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, adminIDKey, id)
}

func AdminIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(adminIDKey).(string)
	return v
}
