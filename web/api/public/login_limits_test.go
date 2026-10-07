package public

import (
	"fmt"
	"testing"
	"time"
)

func TestLoginLimiterPrunesCapacityAndSeparatesKeys(t *testing.T) {
	loginAttemptsMu.Lock()
	old, oldPrune := loginAttempts, lastLoginPrune
	loginAttempts = make(map[string]*loginAttemptInfo)
	lastLoginPrune = time.Time{}
	for i := 0; i < 10000; i++ {
		loginAttempts[fmt.Sprint(i)] = &loginAttemptInfo{count: 5, lastAttempt: time.Now().Add(-16 * time.Minute), lockedUntil: time.Now().Add(-time.Minute)}
	}
	loginAttemptsMu.Unlock()
	defer func() {
		loginAttemptsMu.Lock()
		loginAttempts, lastLoginPrune = old, oldPrune
		loginAttemptsMu.Unlock()
	}()
	if err := checkLoginRateLimit("new-ip"); err != nil {
		t.Fatal("expired capacity never recovered", err)
	}
	for i := 0; i < 5; i++ {
		recordLoginFailure("account:hash")
	}
	if err := checkLoginRateLimit("account:hash"); err == nil {
		t.Fatal("account rate limit missing")
	}
	if err := checkLoginRateLimit("new-ip"); err != nil {
		t.Fatal("independent IP was blocked", err)
	}
	recordLoginSuccess("account:hash")
	if err := checkLoginRateLimit("account:hash"); err != nil {
		t.Fatal("success did not clear limiter", err)
	}
}
