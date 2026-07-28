package auth

import "testing"

// The magic-link endpoint must never leak the login code to the caller in
// production. Returning it is a full authentication bypass: anyone who knows a
// user's email can mint a session in two unauthenticated requests. The code is
// only echoed back when dev codes are explicitly enabled (local dev and E2E).
func TestMagicLinkResponse(t *testing.T) {
	const code = "deadbeef"

	t.Run("omits the code by default", func(t *testing.T) {
		resp := magicLinkResponse(code, false)

		if _, leaked := resp["code"]; leaked {
			t.Fatalf("code leaked in production response: %v", resp)
		}
		if got := resp["status"]; got != "sent" {
			t.Errorf("status = %q, want %q", got, "sent")
		}
	})

	t.Run("includes the code when dev codes are enabled", func(t *testing.T) {
		resp := magicLinkResponse(code, true)

		if got := resp["code"]; got != code {
			t.Errorf("code = %q, want %q", got, code)
		}
		if got := resp["status"]; got != "sent" {
			t.Errorf("status = %q, want %q", got, "sent")
		}
	})
}
