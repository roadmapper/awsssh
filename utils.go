package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/go-ini/ini"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/user"
)

const loadBalancedProxyUrl = "http://proxy.example.com"

// Gets the appropriate bastion host, could pass in environment specific data for this
func GetProxyJumpBastion() string {
	return "bastion.host.example.com"
}

// Modified from the AWS Go SDK: https://github.com/aws/aws-sdk-go/blob/master/aws/credentials/shared_credentials_provider.go#L117
// getCredentialsFilename returns the shared credentials file name to use to read AWS shared credentials.
//
// Will return an error if the user's home directory path cannot be found.
func GetCredentialsFilename() (string, error) {
	var filename string
	if filename = os.Getenv("AWS_SHARED_CREDENTIALS_FILE"); len(filename) != 0 {
		return filename, nil
	}

	if home := UserHomeDir(); len(home) == 0 {
		// Backwards compatibility of home directly not found error being returned.
		// This error is too verbose, failure when opening the file would of been
		// a better error to return.
		return "", awserr.New("UserHomeNotFound", "user home directory not found.", nil)
	}

	filename = SharedCredentialsFilename()

	return filename, nil
}

// Modified from the AWS Go SDK: https://github.com/aws/aws-sdk-go/blob/master/aws/credentials/shared_credentials_provider.go#L117
// getCredentialsFilename returns the shared credentials file name to use to read AWS shared credentials.
//
// Will return an error if the user's home directory path cannot be found.
func GetConfigFilename() (string, error) {
	var filename string
	if filename = os.Getenv("AWS_CONFIG_FILE"); len(filename) != 0 {
		return filename, nil
	}

	if home := UserHomeDir(); len(home) == 0 {
		// Backwards compatibility of home directly not found error being returned.
		// This error is too verbose, failure when opening the file would of been
		// a better error to return.
		return "", awserr.New("UserHomeNotFound", "user home directory not found.", nil)
	}

	filename = SharedConfigFilename()

	return filename, nil
}

// Gets a HTTP client that has the load balanced proxy set
func GetProxyHttpClient() http.Client {
	//proxyUrl, _ := url.Parse(loadBalancedProxyUrl)
	transport := http.Transport{
		//Proxy: http.ProxyURL(proxyUrl),
	}
	client := http.Client{
		Transport: &transport,
	}
	return client
}

// Gets the currently logged in user from the system.
func GetUser() string {
	usr, err := user.Current()
	if err != nil {
		// if we are unable to get the current user, there is something very wrong
		panic(err)
	}
	return usr.Username
}

// Reauthenticate to the proxy, unimplemented due to differing standards for this
func ReauthProxy() bool {
	fmt.Println("Proxy authentication required!")
	return true
}

// Get AWS STS tokens (could implement using by calling custom federation broker for SSO)
func RefreshTokens(selectedProfile string) bool {
	filename, filenameErr := GetConfigFilename()
	if filenameErr != nil {
		panic(`AWS config not found at ${HOME}/.aws/config.`)
	}

	sections, iniErr := ini.Load(filename)
	if iniErr != nil {
		panic(`AWS config could not be read.`)
	}

	sectionName := "profile " + selectedProfile
	roleArnString := sections.Section(sectionName).Key("saml_role").String()

	if roleArnString == "" {
		panic(`No AWS role ARN found.`)
	}

	_, arnErr := arn.Parse(roleArnString)
	if arnErr != nil {
		panic(fmt.Sprintf(`%s is an invalid ARN`, roleArnString))
	}

	tokensRetrieved := false
	return tokensRetrieved
}

// Check if it is possible to connect to AWS
func VerifyAwsConnectivity() bool {
	client := GetProxyHttpClient()
	result, err := client.Get("https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/index.json")
	if err != nil {
		opErr, ok := err.(*url.Error)
		if ok {
			oErr, ok := opErr.Err.(*net.OpError)
			if ok {
				if oErr.Op == "proxyconnect" {
					if !ReauthProxy() {
						fmt.Println("Unable to connect to the proxy, need to turn proxy on")
						return false
					}
				}
			}
			if opErr.Err.Error() == "Proxy Authentication Required" {
				if !ReauthProxy() {
					fmt.Println("Unable to connect to the proxy, need to turn proxy on")
					return false
				}
			}
		}
	}
	if result != nil && result.StatusCode == 407 {
		if !ReauthProxy() {
			fmt.Println("Unable to connect to the proxy, need to turn proxy on")
			return false
		}
	}
	return true
}
