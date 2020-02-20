package stx

import (
	"os"
	"strings"

	"go.mozilla.org/sops/v3/decrypt"
)

// Secrets is a map of decrypted key:value pairs
type Secrets [][]string

// DecryptSecrets uses sops to decrypt the file with credentials from the given profile
func DecryptSecrets(file, profile string) (Secrets, error) {
	credentials := GetProfileCredentials(profile)
	// set ENV vars (primarily for sops decrypt)
	os.Setenv("AWS_ACCESS_KEY_ID", credentials.AccessKeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", credentials.SecretAccessKey)
	os.Setenv("AWS_SESSION_TOKEN", credentials.SessionToken)
	sopsOutput, sopsError := decrypt.File(file, "Dotenv")

	// TODO check error
	var secrets Secrets
	// sops output is key=value\n so first split on new line
	sopsLines := strings.Split(string(sopsOutput), "\n")

	for _, sopLine := range sopsLines {
		// split on =
		if len(sopLine) > 0 {
			sopsPair := strings.Split(sopLine, "=")
			secrets = append(secrets, sopsPair)
		}
	}

	return secrets, sopsError
}
