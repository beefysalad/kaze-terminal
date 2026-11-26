package control

import "crypto/hmac"

// OwnerKeyHost supplies private keys for currently open owner-created tabs.
// A worker, closed tab, name or unknown ID must return an empty key.
type OwnerKeyHost interface {
	OwnerKey(id string) string
}

// RequiresOwnerKey reserves worker-management operations before their handlers
// are added. Ordinary control operations and hook messages need no caller key.
// HandleContext also verifies owner keys for wait requests with children set.
func RequiresOwnerKey(op string) bool {
	return op == "spawn" || op == "kill"
}

// VerifyOwnerKey checks a stable tab ID, never a caller-supplied name or alias.
// All refusals have the same public code and message.
func VerifyOwnerKey(host Host, from, key string) *Error {
	expected := ""
	if h, ok := host.(OwnerKeyHost); ok {
		expected = h.OwnerKey(from)
	}
	equal := hmac.Equal([]byte(expected), []byte(key))
	if expected == "" || !equal {
		return &Error{CodeNotAllowed, "caller is not allowed"}
	}
	return nil
}
