package config

import "github.com/zalando/go-keyring"

const (
	keyringService    = "jiru"
	keyringServiceOld = "jiratui"
	keyringUser       = "api-token"
)

// setKeyringToken stores the API token in the OS keychain.
func setKeyringToken(token string) error {
	return keyring.Set(keyringService, keyringUser, token)
}

// getKeyringToken retrieves the API token from the OS keychain.
// Falls back to the old "jiratui" service name and migrates if found.
func getKeyringToken() (string, error) {
	token, err := keyring.Get(keyringService, keyringUser)
	if err == nil {
		return token, nil
	}
	// Try the old service name.
	token, err = keyring.Get(keyringServiceOld, keyringUser)
	if err != nil {
		return "", err
	}
	// Migrate to new service name.
	_ = keyring.Set(keyringService, keyringUser, token)
	_ = keyring.Delete(keyringServiceOld, keyringUser)
	return token, nil
}
