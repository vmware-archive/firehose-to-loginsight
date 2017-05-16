package loginsight

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/cloudfoundry-community/firehose-to-syslog/logging"
)

type Forwarder struct {
	LogInsightReservedFields []string
	url                      *string
	hasJSONLogMsg            bool
	debug                    bool
	client                   *http.Client
}

//NewForwarder - Creates new instance of LogInsight that implments logging.Logging interface
func NewForwarder(logInsightServer string, logInsightPort int, logInsightReservedFields, logInsightAgentID string, logInsightHasJsonLogMsg, debugging bool) logging.Logging {

	url := fmt.Sprintf("https://%s:%d/api/v1/messages/ingest/%s", logInsightServer, logInsightPort, logInsightAgentID)
	logging.LogStd(fmt.Sprintf("Using %s for log insight", url), true)
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
		TLSClientConfig:    &tls.Config{InsecureSkipVerify: true},
	}
	return &Forwarder{
		LogInsightReservedFields: strings.Split(logInsightReservedFields, ","),
		url:           &url,
		hasJSONLogMsg: logInsightHasJsonLogMsg,
		debug:         debugging,
		client:        &http.Client{Transport: tr},
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
	go func() {
		messages := Messages{}
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

		if f.hasJSONLogMsg {

			var obj map[string]interface{}
			msgbytes := []byte(msg)
			err := json.Unmarshal(msgbytes, &obj)
			if err == nil {
				for k, v := range obj {
					message.Fields = append(message.Fields, Field{Name: f.CreateKey(k), Content: fmt.Sprint(v)})
				}
			} else {
				logging.LogError("Error unmarshalling", err)
				return
			}
		}

		messages.Messages = append(messages.Messages, message)
		payload, err := json.Marshal(messages)
		if err == nil {
			f.Post(*f.url, payload)
		} else {
			logging.LogError("Error marshalling", err)
		}
	}()
}

func (f *Forwarder) Post(url string, payload []byte) {
	if f.debug {
		logging.LogStd("Post being sent", true)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		logging.LogError("Error building request", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		logging.LogError("Error Posting data", err)
		return
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	if f.debug {
		logging.LogStd(fmt.Sprintf("Post response code %s with body %s", resp.Status, body), true)
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
