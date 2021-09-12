package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/go-ini/ini"
	"github.com/blang/semver/v4"
	"github.com/urfave/cli"
	"gopkg.in/AlecAivazis/survey.v1"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type Instance struct {
	Name     string
	Instance *ec2.Instance
}

var instances []*Instance
var instancesStrings []string

var ipRegex = regexp.MustCompile(`(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}`)
var releasesPage = "https://github.com/roadmapper/awsssh/releases/latest"

func evaluateAppVersion(versionString string) {
	var latestVersionString = versionString
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest(http.MethodGet,
		"https://github.com/roadmapper/awsssh/version.json", nil)
	if req != nil {
		req.Header.Add("Connection", "keep-alive")
		req.Header.Add("Accept", "application/json")
	}
	resp, err := client.Do(req)

	if err != nil {
		fmt.Printf("The HTTP request failed with error %s\n", err)
		fmt.Printf("Unable to get app version data from GitHub Pages")
	} else {
		byteData, _ := io.ReadAll(resp.Body)
		if (resp.StatusCode != 200) {
			fmt.Println("The HTTP request failed with error", string(byteData))
			fmt.Println("Unable to get app version data from GitHub Pages")
		} else {
			var data map[string]interface{}
			jsonErr := json.Unmarshal(byteData, &data)

			if jsonErr != nil {
				fmt.Printf("JSON parsing from GitHub Pages failed.")
			}
			latestVersionString = data["version"].(string)
		}
	}
	appVersion, _ := semver.Make(versionString)
	latestAppVersion, _ := semver.Make(latestVersionString)

	if appVersion.LT(latestAppVersion) {
		fmt.Printf("Update available! Download %s from GitHub: %s\n\n", latestVersionString, releasesPage)
	}
}

func getConnectionString(user string, host *string) *string {
	connectionString := fmt.Sprintf("%s@%s", user, *host)
	return &connectionString
}

func ssh(user string, connectionString *string) {
	cmd := exec.Command("ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		*connectionString)
	proxyJumpArg := fmt.Sprintf("-o ProxyJump=%s@%s", user, GetProxyJumpBastion())
	cmd.Args = append(cmd.Args, proxyJumpArg)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
	}
}

func main() {
	var profile string
	var region = ""
	var query string
	var instance string

	app := cli.NewApp()
	app.Name = "awsssh"
	app.Version = "1.0.9"
	app.Usage = "SSH into AWS EC2 instances"
	app.Authors = []cli.Author{
		{
			Name:  "Vinay Dandekar",
			Email: "vindansam@hotmail.com",
		},
	}
	app.ArgsUsage = "[hostname or IP address (optional)]"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "profile",
			Usage:       "the AWS profile to use when connecting",
			Destination: &profile,
			EnvVar:      "AWS_DEFAULT_PROFILE",
		},
		cli.StringFlag{
			Name:        "region, r",
			Usage:       "AWS region",
			Value:       "us-east-1",
			Destination: &region,
			EnvVar:      "AWS_DEFAULT_REGION",
		},
		cli.StringFlag{
			Name:        "query, q",
			Usage:       "query that will search by the name tag",
			Destination: &query,
		},
		cli.StringFlag{
			Name:        "instance, i",
			Usage:       "instance ID",
			Destination: &instance,
		},
	}

	app.Action = func(c *cli.Context) error {
		evaluateAppVersion(app.Version)

		if c.NArg() < 1 {
			fmt.Println(`No arguments found.`)
			os.Exit(1)
		}

		host := c.Args().First()

		if host != "" { // hostname/IP passed in
			if ipRegex.MatchString(host) {
				parsed := net.ParseIP(host)
				if parsed == nil {
					fmt.Println("IP address not valid.")
					os.Exit(1)
				}
			} else {
				resolvedIPs, err := net.LookupHost(host)
				if err != nil {
					fmt.Println("Unable to resolve the given hostname.")
					os.Exit(1)
				}
				host = resolvedIPs[0] // Assume that the first IP address of the DNS lookup is the destination
			}
		} else { // search AWS for instances
			filename, filenameErr := GetCredentialsFilename()
			if filenameErr != nil {
				fmt.Println(`AWS credentials not found at ${HOME}/.aws/credentials.`)
				os.Exit(1)
			}

			sections, iniErr := ini.Load(filename)
			if iniErr != nil {
				fmt.Println(`AWS credentials could not be read.`)
				os.Exit(1)
			}

			if profile == "" {
				var profiles []string

				for _, section := range sections.Sections() {
					profileName := section.Name()
					if profileName != "DEFAULT" {
						profiles = append(profiles, profileName)
					}
				}

				if profiles == nil {
					fmt.Println(`No AWS profiles found.`)
					os.Exit(1)
				}

				var profileQuestion = []*survey.Question{
					{
						Name: "profile",
						Prompt: &survey.Select{
							Message: "Select an AWS profile:",
							Options: profiles,
						},
					},
				}

				// the answers will be written to this struct
				profileAnswer := struct {
					Profile string
				}{}

				// ask the questions
				answerErr := survey.Ask(profileQuestion, &profileAnswer)
				if answerErr != nil {
					return nil
				}
				profile = profileAnswer.Profile
			}

			tokenExpiration, _ := time.Parse(time.RFC3339,
				sections.Section(profile).Key("token_expiration").String())
			if time.Now().After(tokenExpiration) {
				if !RefreshTokens(profile) {
					fmt.Println("Refresh your credentials via the AWS CLI!")
					os.Exit(1)
				}
			}

			if !VerifyAwsConnectivity() {
				fmt.Println("Unable to reach amazonaws.com")
				os.Exit(1)
			}

			client := GetProxyHttpClient()

			// Specify profile for config and region for requests
			sess := session.Must(session.NewSessionWithOptions(session.Options{
				Config: aws.Config{
					Region:     aws.String(region),
					HTTPClient: &client,
				},
				Profile: profile,
			}))

			_, credentialsErr := sess.Config.Credentials.Get()
			if credentialsErr != nil {
				fmt.Println("No credentials found!")
				os.Exit(1)
			}

			// As of 2020-05-19, Credentials.IsExpired() will always return false
			// https://github.com/aws/aws-sdk-go/blob/6b25fdfe03b0477dd3c77f317703724eab772653/aws/credentials/shared_credentials_provider.go#L71-L74

			var filters []*ec2.Filter

			if query != "" {
				filters = append(filters, &ec2.Filter{
					Name:   aws.String("tag:Name"),
					Values: []*string{aws.String("*" + query + "*")},
				})
			}

			if instance != "" {
				filters = append(filters, &ec2.Filter{
					Name:   aws.String("instance-id"),
					Values: []*string{aws.String(instance)},
				})
			}

			svc := ec2.New(sess)
			input := &ec2.DescribeInstancesInput{
				Filters: filters,
			}

			// TODO: provide pagination support
			result, err := svc.DescribeInstances(input)
			if err != nil {
				if aerr, ok := err.(awserr.Error); ok {
					switch aerr.Code() {
					default:
						fmt.Println(aerr.Error())
					}
				} else {
					// Print the error, cast err to awserr.Error to get the Code and
					// Message from an error.
					fmt.Println(err.Error())
				}
			}

			for _, reservation := range result.Reservations {
				for _, instance := range reservation.Instances {
					if *instance.State.Name == "running" {
						for _, tag := range instance.Tags {
							if *tag.Key == "Name" {
								name := *tag.Value
								instances = append(instances, &Instance{
									Name:     name,
									Instance: instance,
								})
							}
						}
					}
				}
			}

			if len(instances) == 0 {
				fmt.Printf("No running instances found!\n")
				os.Exit(1)
			}

			for _, instance := range instances {
				var formattedString = fmt.Sprintf("%s\t%s\t%s\t%s", instance.Name, *instance.Instance.InstanceId,
					*instance.Instance.PrivateIpAddress, instance.Instance.LaunchTime.String())
				instancesStrings = append(instancesStrings, formattedString)
			}

			// the questions to ask
			var instanceQuestion = []*survey.Question{
				{
					Name: "selection",
					Prompt: &survey.Select{
						Message: "Select an EC2 instance:",
						Options: instancesStrings,
					},
				},
			}

			instanceAnswer := struct {
				Selection string
			}{}

			// ask the questions
			answerErr := survey.Ask(instanceQuestion, &instanceAnswer)
			if answerErr != nil {
				return nil
			}

			for _, instance := range instances {
				if strings.Contains(instanceAnswer.Selection, *instance.Instance.PrivateIpAddress) {
					host = *instance.Instance.PrivateIpAddress
					break
				}
			}
		}

		if host != "" {
			user := GetUser()
			connectionString := getConnectionString(user, &host)
			ssh(user, connectionString)
		}

		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
