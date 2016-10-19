package loginsight

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/cloudfoundry-community/firehose-to-syslog/logging"
)

type LogInsight struct {
	LogInsightServer      *string
	LogInsightPort        *int
	LogInsightBatchSize   *int
	LogInsightFieldPrefix *string
	LogInsightAgentID     *string
	Messages              Messages
}

//NewLogging - Creates new instance of LogInsight that implments logging.Logging interface
func NewLogging(logInsightServer *string, logInsightPort, logInsightBatchSize *int, logInsightFieldPrefix *string, logInsightAgentID *string) logging.Logging {
	return &LogInsight{
		LogInsightServer:      logInsightServer,
		LogInsightPort:        logInsightPort,
		LogInsightBatchSize:   logInsightBatchSize,
		LogInsightFieldPrefix: logInsightFieldPrefix,
		LogInsightAgentID:     logInsightAgentID,
		Messages:              Messages{},
	}
}

func (l *LogInsight) Connect() bool {
	return true
}

func (l *LogInsight) ShipEvents(eventFields map[string]interface{}, msg string) {
	message := Message{
		Text: msg,
	}
	for k, v := range eventFields {
		if k == "timestamp" {
			message.Timestamp = v.(int64)
		} else {
			message.Fields = append(message.Fields, Field{Name: *l.LogInsightFieldPrefix + k, Content: fmt.Sprint(v)})
		}
	}
	l.Messages.Messages = append(l.Messages.Messages, message)

	if len(l.Messages.Messages) >= *l.LogInsightBatchSize {
		if jsonstr, err := json.Marshal(l.Messages); err == nil {
			url := fmt.Sprintf("https://%s:%d/api/v1/messages/ingest/%s", *l.LogInsightServer, *l.LogInsightPort, *l.LogInsightAgentID)
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
			var req *http.Request
			var resp *http.Response
			if req, err = http.NewRequest("POST", url, bytes.NewBuffer(jsonstr)); err == nil {
				req.Header.Add("Content-Type", "application/json")
				client := &http.Client{Transport: tr}
				if resp, err = client.Do(req); err == nil {
					defer resp.Body.Close()
					if resp.StatusCode != 200 {
						body, _ := ioutil.ReadAll(resp.Body)
						logging.LogError(fmt.Sprintf("Error posting data to log insight with status %d", resp.StatusCode), string(body))
						fmt.Println("response Status:", resp.Status)
						fmt.Println("response Headers:", resp.Header)
						fmt.Println("response Body:", string(body))
					} else {
						l.Messages = Messages{}
					}
				} else {
					logging.LogError("Error sending data", err)
				}
			} else {
				logging.LogError("Error creating request", err)
			}
		} else {
			logging.LogError("Error marshalling", err)
		}
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
