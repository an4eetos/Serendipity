// Package ports declares what the identity context needs from the outside.
package ports

// TokenVerifier checks access tokens.
type TokenVerifier interface {
	// Verify returns the user id of a valid access token.
	Verify(token string) (userID string, err error)
}
