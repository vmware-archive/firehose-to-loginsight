package loginsight

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/parnurzeal/gorequest"

	"github.com/cloudfoundry-community/firehose-to-syslog/logging"
)

type Forwarder struct {
	LogInsightBatchSize      int
	LogInsightReservedFields []string
	Messages                 Messages
	url                      *string
	hasJsonLogMsg            bool
}

//NewForwarder - Creates new instance of LogInsight that implments logging.Logging interface
func NewForwarder(logInsightServer string, logInsightPort, logInsightBatchSize int, logInsightReservedFields, logInsightAgentID, logInsightHasJsonLogMsg string) *Forwarder {
	b, err := strconv.ParseBool(logInsightHasJsonLogMsg)
	if err != nil {
		b = false
	}

	url := fmt.Sprintf("https://%s:%d/api/v1/messages/ingest/%s", logInsightServer, logInsightPort, logInsightAgentID)

	return &Forwarder{
		LogInsightBatchSize:      logInsightBatchSize,
		LogInsightReservedFields: strings.Split(logInsightReservedFields, ","),
		Messages:                 Messages{},
		url:                      &url,
		hasJsonLogMsg:            b,
	}
}

func (f *Forwarder) Connect() bool {
	return true
}

func (f *Forwarder) CreateKey(k string) string {
	if contains(f.LogInsightReservedFields, k) {
		return "cf_" + k
	} else {
		return k
	}
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func (f *Forwarder) ShipEvents(eventFields map[string]interface{}, msg string) {

	message := Message{
		Text: msg,
	}

	for k, v := range eventFields {
		if k == "timestamp" {
			message.Timestamp = v.(int64)
		} else {
			message.Fields = append(message.Fields, Field{Name: f.CreateKey(k), Content: fmt.Sprint(v)})
		}
	}

	if f.hasJsonLogMsg {

		var obj interface{}
		msgbytes := []byte(msg)
		err := json.Unmarshal(msgbytes, &obj)
		if err == nil {

			for k, v := range obj.(map[string]interface{}) {
				message.Fields = append(message.Fields, Field{Name: f.CreateKey(k), Content: fmt.Sprint(v)})
			}
		}

		msgbytes = nil
		f = nil

	}
	f.Messages.Messages = append(f.Messages.Messages, message)
	if len(f.Messages.Messages) >= f.LogInsightBatchSize {
		payload, err := json.Marshal(f.Messages)
		if err == nil {
			f.Post(*f.url, string(payload))
		} else {
			logging.LogError("Error marshalling", err)
		}
		message.Fields = nil
		f.Messages.Messages = nil
	}
}

func (l *Forwarder) Post(url, payload string) {
	request := gorequest.New()
	post := request.Post(url)
	post.TLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	post.Set("Content-Type", "application/json")
	post.Send(payload)
	res, body, errs := post.End()
	if len(errs) > 0 {
		logging.LogError("Error Posting data", errs[0])
	}
	if res.StatusCode != http.StatusOK {
		logging.LogError("non 200 status code", fmt.Errorf("Status %d, body %s", res.StatusCode, body))
	}
}

type Messages struct {
	Messages []Message `json:"messages"`
}

type Message struct {
	Fields    []Field `json:"fields"`
	Text      string  `json:"text"`
	Timestamp int64   `json:"timestamp"`
}

type Field struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}
