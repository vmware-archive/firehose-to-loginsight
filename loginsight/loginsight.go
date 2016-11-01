package loginsight

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/cloudfoundry-community/firehose-to-syslog/logging"
)

type LogInsight struct {
	LogInsightBatchSize      *int
	LogInsightReservedFields []string
	Messages                 Messages
	url                      *string
	client                   *http.Client
}

//NewLogging - Creates new instance of LogInsight that implments logging.Logging interface
func NewLogging(logInsightServer *string, logInsightPort, logInsightBatchSize *int, logInsightReservedFields *string, logInsightAgentID *string) logging.Logging {

	baseClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	url := fmt.Sprintf("https://%s:%d/api/v1/messages/ingest/%s", *logInsightServer, *logInsightPort, *logInsightAgentID)

	return &LogInsight{
		LogInsightBatchSize:      logInsightBatchSize,
		LogInsightReservedFields: strings.Split(*logInsightReservedFields, ","),
		Messages:                 Messages{},
		client:                   baseClient,
		url:                      &url,
	}
}

func (l *LogInsight) Connect() bool {
	return true
}

func (l *LogInsight) CreateKey(k string) string {
	if contains(l.LogInsightReservedFields, k) {
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

func (l *LogInsight) ShipEvents(eventFields map[string]interface{}, msg string) {
	message := Message{
		Text: msg,
	}
	for k, v := range eventFields {
		if k == "timestamp" {
			message.Timestamp = v.(int64)
		} else {
			message.Fields = append(message.Fields, Field{Name: l.CreateKey(k), Content: fmt.Sprint(v)})
		}
	}

	l.Messages.Messages = append(l.Messages.Messages, message)
	if len(l.Messages.Messages) >= *l.LogInsightBatchSize {

		jsonBuffer := new(bytes.Buffer)
		encoder := json.NewEncoder(jsonBuffer)

		if err := encoder.Encode(l.Messages); err == nil {
			l.sendToLogInsight(jsonBuffer)
		} else {
			logging.LogError("Error marshalling", err)
		}

		jsonBuffer = nil
		encoder = nil
	}

	message.Fields = nil
	l.Messages.Messages = nil
}

func (l *LogInsight) sendToLogInsight(jsonBuffer *bytes.Buffer) {
	if req, err := http.NewRequest("POST", *l.url, jsonBuffer); err == nil {
		req.Header.Add("Content-Type", "application/json")

		if resp, err := l.client.Do(req); err == nil {
			defer resp.Body.Close()
			_, _ = ioutil.ReadAll(resp.Body)
		} else {
			logging.LogError("Error sending data", err)
		}
	} else {
		logging.LogError("Error creating request", err)
	}
	jsonBuffer = nil
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
