package security

import "context"

type Principal struct {
	UserID      int64
	UserName    string
	UserEmail   string
	GlobalRoles []string
}

type principalKey struct{}

// PrincipalGinKey is also stored in gin.Context so application services that
// receive gin.Context as context.Context can resolve the authenticated user.
const PrincipalGinKey = "fluxa.security.principal"

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	if principal, ok := ctx.Value(principalKey{}).(Principal); ok {
		return principal, true
	}
	principal, ok := ctx.Value(PrincipalGinKey).(Principal)
	return principal, ok
}
