package stx

import (
	"bytes"
	"os"

	"github.com/joho/godotenv"
	"go.mozilla.org/sops/v3/decrypt"
)

// DecryptSecrets uses sops to decrypt the file with credentials from the given profile
func DecryptSecrets(file, profile string) (map[string]string, error) {
	credentials := GetProfileCredentials(profile)
	// set ENV vars (primarily for sops decrypt)
	os.Setenv("AWS_ACCESS_KEY_ID", credentials.AccessKeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", credentials.SecretAccessKey)
	os.Setenv("AWS_SESSION_TOKEN", credentials.SessionToken)
	sopsOutput, sopsError := decrypt.File(file, "Dotenv")

	secrets, _ := godotenv.Parse(bytes.NewReader(sopsOutput))

	return secrets, sopsError
}
