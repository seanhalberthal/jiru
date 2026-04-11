package config

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "jiru"
	keyringUser    = "api-token"
)

// keyringUserForProfile returns the profile-aware keyring user key.
func keyringUserForProfile(profile string) string {
	if profile == "" || profile == "default" {
		return keyringUser // backward compat
	}
	return keyringUser + "-" + profile
}

// deleteKeyringToken removes the API token from the OS keychain (legacy default key).
func deleteKeyringToken() {
	_ = keyring.Delete(keyringService, keyringUser)
}

// setKeyringTokenForProfile stores a token for a named profile.
func setKeyringTokenForProfile(profile, token string) error {
	return friendlyKeyringError(keyring.Set(keyringService, keyringUserForProfile(profile), token))
}

// getKeyringTokenForProfile retrieves a token for a named profile.
func getKeyringTokenForProfile(profile string) (string, error) {
	token, err := keyring.Get(keyringService, keyringUserForProfile(profile))
	return token, friendlyKeyringError(err)
}

// deleteKeyringTokenForProfile removes a token for a named profile.
func deleteKeyringTokenForProfile(profile string) {
	_ = keyring.Delete(keyringService, keyringUserForProfile(profile))
}

// friendlyKeyringError wraps an error returned by the OS keychain with a
// message that tells the user how to recover from common failures.
//
// On macOS the go-keyring library shells out to /usr/bin/security and
// discards stderr, so the only signal we have is the exec exit code — which
// maps to truncated OSStatus values. Exit 36 in particular corresponds to
// errSecInteractionNotAllowed (-25308), typically triggered by a locked
// login keychain or a terminal session that cannot present an auth prompt.
func friendlyKeyringError(err error) error {
	if err == nil || runtime.GOOS != "darwin" {
		return err
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return err
	}
	switch exitErr.ExitCode() {
	case 36:
		return fmt.Errorf("macOS keychain cannot present a prompt (errSecInteractionNotAllowed). " +
			"The login keychain may be locked — run `security unlock-keychain ~/Library/Keychains/login.keychain-db` " +
			"or open Keychain Access.app and unlock 'login', then retry")
	case 51:
		return fmt.Errorf("macOS keychain authorisation failed (errSecAuthFailed). " +
			"Check that your user has access to the login keychain")
	default:
		return fmt.Errorf("macOS keychain error (security exit %d): %w", exitErr.ExitCode(), err)
	}
}
