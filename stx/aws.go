package stx

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/logrusorgru/aurora"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
)

// EnsureVaultSession is used to prompt for MFA if aws-vault session has expired
func EnsureVaultSession(config *Config) {
	au := aurora.NewAurora(true)
	_, existingVault := os.LookupEnv("AWS_VAULT")
	if existingVault {
		fmt.Println(au.Red("Cannot run in nested aws-vault session!"))
		os.Exit(1)
	}

	sessionsOut, sessionsErr := exec.Command("aws-vault", "list", "--sessions").Output()
	if sessionsErr != nil {
		fmt.Println("Could not list aws-vault sessions: " + au.Red(sessionsErr).String())
		os.Exit(1)
	}
	//fmt.Println(string(sessionsOut))
	var mfa string
	if len(sessionsOut) < 1 {
		if len(config.Auth.Ykman.Profile) > 1 {
			ykmanOutput, ykmanErr := exec.Command("ykman", "oath", "code", "-s", config.Auth.Ykman.Profile).Output()
			if ykmanErr != nil {
				fmt.Println(au.Red(ykmanErr))
				os.Exit(1)
			}
			mfa = strings.TrimSpace(string(ykmanOutput))
			fmt.Println("Pulled MFA from ykman profile ", config.Auth.Ykman.Profile)
		} else {
			fmt.Print("MFA: ")
			fmt.Scanln(&mfa)
		}
		awsVaultExecErr := exec.Command("aws-vault", "exec", "-t", mfa, config.Auth.AwsVault.SourceProfile).Run()
		if awsVaultExecErr != nil {
			fmt.Println(au.Red("aws-vault error: " + awsVaultExecErr.Error()))
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
	au := aurora.NewAurora(true)
	execOut, execErr := exec.Command("aws-vault", "exec", "--json", profile).Output()

	if execErr != nil {
		fmt.Println(au.Red(execErr))
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
