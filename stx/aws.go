package stx

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
)

// EnsureVaultSession is used to prompt for MFA if aws-vault session has expired
func EnsureVaultSession() {
	sessionsOut, _ := exec.Command("aws-vault", "list", "--sessions").Output()
	//fmt.Println(string(sessionsOut))
	if len(sessionsOut) < 1 {
		fmt.Print("MFA: ")
		var input string
		fmt.Scanln(&input)
		err := exec.Command("aws-vault", "exec", "-t", input, "gloo-users").Run()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
}

type awsCredentials struct {
	AccessKeyID                               string `json:"AccessKeyId"`
	SecretAccessKey, SessionToken, Expiration string
}

func getProfileCredentials(profile string) awsCredentials {
	execOut, execErr := exec.Command("aws-vault", "exec", "--json", profile).Output()

	if execErr != nil {
		fmt.Println(execErr)
		os.Exit(1)
	}
	// TODO: cache credentials until expired
	var credentials awsCredentials
	json.Unmarshal(execOut, &credentials)
	return credentials
}

// GetSession returns aws session with credentials from profile
func GetSession(profile string) *session.Session {
	creds := getProfileCredentials(profile)
	config := aws.NewConfig().WithCredentials(credentials.NewStaticCredentials(creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken))
	sess, _ := session.NewSession(config)
	return sess
}
