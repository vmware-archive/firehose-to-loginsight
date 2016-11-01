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
	LogInsightServer         *string
	LogInsightPort           *int
	LogInsightBatchSize      *int
	LogInsightReservedFields []string
	LogInsightAgentID        *string
	Messages                 Messages
	LogInsightClient         *http.Client
}

//NewLogging - Creates new instance of LogInsight that implments logging.Logging interface
func NewLogging(logInsightServer *string, logInsightPort, logInsightBatchSize *int, logInsightReservedFields *string, logInsightAgentID *string) logging.Logging {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	baseClient := &http.Client{Transport: tr}

	return &LogInsight{
		LogInsightServer:         logInsightServer,
		LogInsightPort:           logInsightPort,
		LogInsightBatchSize:      logInsightBatchSize,
		LogInsightReservedFields: strings.Split(*logInsightReservedFields, ","),
		LogInsightAgentID:        logInsightAgentID,
		Messages:                 Messages{},
		LogInsightClient:         baseClient,
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
			url := fmt.Sprintf("https://%s:%d/api/v1/messages/ingest/%s", *l.LogInsightServer, *l.LogInsightPort, *l.LogInsightAgentID)

			var req *http.Request
			if req, err = http.NewRequest("POST", url, jsonBuffer); err == nil {
				defer req.Body.Close()

				req.Header.Add("Content-Type", "application/json")
				go l.handleLogInsightResponse(req, jsonBuffer)
			} else {
				logging.LogError("Error creating request", err)
			}
		} else {
			logging.LogError("Error marshalling", err)
		}

		jsonBuffer = nil
		encoder = nil
	}

	message.Fields = nil
	l.Messages.Messages = nil
}

func (l *LogInsight) handleLogInsightResponse(req *http.Request, jsonBuffer *bytes.Buffer) func() {
	return func() {
		if resp, err := l.LogInsightClient.Do(req); err == nil {
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				body, _ := ioutil.ReadAll(resp.Body)
				logging.LogError(fmt.Sprintf("Error posting data to log insight with status %d and payload %s", resp.StatusCode, jsonBuffer.String()), string(body))
				fmt.Println("response Status:", resp.Status)
				fmt.Println("response Headers:", resp.Header)
				fmt.Println("response Body:", string(body))
			} else {
				l.Messages = Messages{}
			}
		} else {
			logging.LogError("Error sending data", err)
		}
		jsonBuffer = nil
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
