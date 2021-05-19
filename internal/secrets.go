package internal

import (
	"os"

	"go.mozilla.org/sops/v3/decrypt"
)

// DecryptSecrets uses sops to decrypt the file with credentials from the given profile
func DecryptSecrets(file, profile string) ([]byte, error) {
	credentials := GetProfileCredentials(profile)
	// set ENV vars (primarily for sops decrypt)
	os.Setenv("AWS_ACCESS_KEY_ID", credentials.AccessKeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", credentials.SecretAccessKey)
	os.Setenv("AWS_SESSION_TOKEN", credentials.SessionToken)
	return decrypt.File(file, "yaml")
}
