package stx

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
)

// EnsureVaultSession is used to prompt for MFA if aws-vault session has expired
func EnsureVaultSession(config Config) {
	sessionsOut, _ := exec.Command("aws-vault", "list", "--sessions").Output()
	//fmt.Println(string(sessionsOut))
	var mfa string
	if len(sessionsOut) < 1 {
		if len(config.Auth.Ykman.Profile) > 1 {
			ykmanOutput, _ := exec.Command("ykman", "oath", "code", "-s", config.Auth.Ykman.Profile).Output()
			mfa = strings.TrimSpace(string(ykmanOutput))
			fmt.Println("Pulled MFA from ykman profile ", config.Auth.Ykman.Profile)
		} else {
			fmt.Print("MFA: ")
			fmt.Scanln(&mfa)
		}
		err := exec.Command("aws-vault", "exec", "-t", mfa, "gloo-users").Run()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
}

// AwsCredentials holds access keys and session token
type AwsCredentials struct {
	AccessKeyID                               string `json:"AccessKeyId"`
	SecretAccessKey, SessionToken, Expiration string
}

// GetProfileCredentials returns AwsCredentials for the given profile
func GetProfileCredentials(profile string) AwsCredentials {
	execOut, execErr := exec.Command("aws-vault", "exec", "--json", profile).Output()

	if execErr != nil {
		fmt.Println(execErr)
		os.Exit(1)
	}
	// TODO: cache credentials until expired
	var credentials AwsCredentials
	json.Unmarshal(execOut, &credentials)

	return credentials
}

// GetSession returns aws session with credentials from profile
func GetSession(profile string) *session.Session {
	creds := GetProfileCredentials(profile)
	config := aws.NewConfig().WithCredentials(credentials.NewStaticCredentials(creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken))
	sess, _ := session.NewSession(config)
	return sess
}
