package internal

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"go.mozilla.org/sops/v3/decrypt"
)

// DecryptSecrets uses sops to decrypt the file with credentials from the given profile
func DecryptSecrets(file, profile string) ([]byte, error) {

	cfg, cfgErr := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(profile))

	if cfgErr != nil {
		log.Fatal(cfgErr)
	}

	creds, credsErr := cfg.Credentials.Retrieve(context.TODO())
	if credsErr != nil {
		log.Fatal(credsErr)
	}

	// set ENV vars (primarily for sops decrypt)
	os.Setenv("AWS_ACCESS_KEY_ID", creds.AccessKeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", creds.SecretAccessKey)
	os.Setenv("AWS_SESSION_TOKEN", creds.SessionToken)
	return decrypt.File(file, "yaml")
}
