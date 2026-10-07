package accounts

import (
	"fmt"
	"sync"
	"time"
)

type sensitiveGrant struct {
	UUID, Factor string
	Until        time.Time
}

var sensitiveMu sync.Mutex
var sensitiveGrants = map[string]sensitiveGrant{}

// A short, session-bound step-up supports a batch of file operations. Changing
// the factor or revoking the session invalidates the grant immediately.
func VerifySensitiveSession(uuid, session, code string, allowGrant bool) error {
	user, err := GetUserByUUID(uuid)
	if err != nil {
		return err
	}
	if user.TwoFactor == "" {
		return nil
	}
	sensitiveMu.Lock()
	grant := sensitiveGrants[session]
	sensitiveMu.Unlock()
	if allowGrant && session != "" && grant.UUID == uuid && grant.Factor == user.TwoFactor && time.Now().Before(grant.Until) {
		owner, err := GetSession(session)
		if err != nil || owner != uuid {
			return fmt.Errorf("invalid session")
		}
		return nil
	}
	if code == "" {
		return fmt.Errorf("2FA code is required")
	}
	valid, err := Verify2Fa(uuid, code)
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("Invalid 2FA code")
	}
	if session != "" {
		// Do not accept a grant for a session belonging to someone else.
		owner, err := GetSession(session)
		if err != nil || owner != uuid {
			return fmt.Errorf("invalid session")
		}
		sensitiveMu.Lock()
		defer sensitiveMu.Unlock()
		for key, value := range sensitiveGrants {
			if time.Now().After(value.Until) {
				delete(sensitiveGrants, key)
			}
		}
		sensitiveGrants[session] = sensitiveGrant{uuid, user.TwoFactor, time.Now().Add(5 * time.Minute)}
	}
	return nil
}
