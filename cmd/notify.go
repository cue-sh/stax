package cmd

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/joho/godotenv"
	"github.com/rdegges/go-ipify"
	"github.com/spf13/cobra"
)

type snsMessage struct {
	Type, Message, Subject, SubscribeURL string
}

func init() {
	rootCmd.AddCommand(notifyCmd)
}

// notifyCmd represents the notify command
var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Listen for stack events sent via SNS.",
	Long: `Notify does not operate on any stack. Instead it creates a very
light-weight http server dedicated to displaying stack events sent through SNS.

To use notify, first start the server by executing the command; no options are 
required. Notify will immediately return the http endpoint on which it is listening.
Copy this URL and use it to configure the EndPoint option (see stx deploy --help)
`,
	Run: func(cmd *cobra.Command, args []string) {
		defer log.Flush()
		ip, ipErr := ipify.GetIp()
		if ipErr != nil {
			log.Fatal(ipErr)
			return
		}

		previousStack := ""

		http.HandleFunc("/notify", func(w http.ResponseWriter, req *http.Request) {
			// log.Infof("Request:\n%+v", req)
			io.WriteString(w, "ok\n")
			messageType := req.Header.Get("x-amz-sns-message-type")
			if messageType != "SubscriptionConfirmation" && messageType != "Notification" {
				return
			}

			bodyBytes, bodyBytesErr := ioutil.ReadAll(req.Body)
			if bodyBytesErr != nil {
				log.Error(bodyBytesErr)
				return
			}

			var message snsMessage
			unmarshalErr := json.Unmarshal(bodyBytes, &message)
			if unmarshalErr != nil {
				log.Error(unmarshalErr)
				return
			}

			switch message.Type {
			case "SubscriptionConfirmation":
				log.Debug("Confirming subscription...")

				_, confirmErr := http.Get(message.SubscribeURL)
				if confirmErr != nil {
					log.Errorf("Could not confirm subscription:\n%s\n", confirmErr)
					return
				}

			case "Notification":

				notification, notificationErr := godotenv.Unmarshal(message.Message)

				if notificationErr != nil {
					log.Error(notificationErr)
					return
				}
				status := notification["ResourceStatus"]
				if strings.Contains(status, "COMPLETE") {
					status = au.BrightGreen(notification["ResourceStatus"]).String()
				}

				if strings.Contains(status, "FAIL") || strings.Contains(status, "ROLLBACK") {
					status = au.Red(notification["ResourceStatus"]).String()
				}

				var stack string
				if previousStack != notification["StackName"] {
					stack = au.Magenta(notification["StackName"]).String()
					previousStack = notification["StackName"]
				} else {
					stack = "    " //strings.Repeat(" ", utf8.RuneCountInString(notification["StackName"]))
				}

				log.Infof("%s %s %s %s\n", stack, notification["LogicalResourceId"], status, notification["ResourceStatusReason"])
			}

			// fmt.Printf("%s", bodyBytes)

		})

		log.Info("Listening on", "http://"+ip+":8080/notify")
		http.ListenAndServe(":8080", nil)
	},
}

// example notification
// {
//   "Type" : "Notification",
//   "MessageId" : "22b80b92-fdea-4c2c-8f9d-bdfb0c7bf324",
//   "TopicArn" : "arn:aws:sns:us-west-2:123456789012:MyTopic",
//   "Subject" : "My First Message",
//   "Message" : "Hello world!",
//   "Timestamp" : "2012-05-02T00:54:06.655Z",
//   "SignatureVersion" : "1",
//   "Signature" : "EXAMPLEw6JRN...",
//   "SigningCertURL" : "https://sns.us-west-2.amazonaws.com/SimpleNotificationService-f3ecfb7224c7233fe7bb5f59f96de52f.pem",
//   "UnsubscribeURL" : "https://sns.us-west-2.amazonaws.com/?Action=Unsubscribe&SubscriptionArn=arn:aws:sns:us-west-2:123456789012:MyTopic:c9135db0-26c4-47ec-8998-413945fb5a96"
// }
