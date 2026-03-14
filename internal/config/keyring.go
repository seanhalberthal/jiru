package config

import "github.com/zalando/go-keyring"

const (
	keyringService = "jiratui"
	keyringUser    = "api-token"
)

// setKeyringToken stores the API token in the OS keychain.
func setKeyringToken(token string) error {
	return keyring.Set(keyringService, keyringUser, token)
}

// getKeyringToken retrieves the API token from the OS keychain.
func getKeyringToken() (string, error) {
	return keyring.Get(keyringService, keyringUser)
}
