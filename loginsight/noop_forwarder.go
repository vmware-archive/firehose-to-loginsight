package loginsight

import (
	"github.com/cloudfoundry-community/firehose-to-syslog/logging"
)

type NoopForwarder struct {
}

//NewNoopForwarder - Creates new instance of a Noop Logger that implments logging.Logging interface
func NewNoopForwarder() logging.Logging {
	return &NoopForwarder{}
}

func (f *NoopForwarder) Connect() bool {
	return true
}
func (f *NoopForwarder) ShipEvents(map[string]interface{}, string) {

}
