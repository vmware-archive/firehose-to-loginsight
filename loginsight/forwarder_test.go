package loginsight_test

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cloudfoundry-community/firehose-to-syslog/logging"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/ghttp"
	"github.com/pivotalservices/firehose-to-loginsight/loginsight"
)

var _ = Describe("Logger", func() {
	var (
		logger *loginsight.Forwarder
	)

	BeforeEach(func() {
		logger = &loginsight.Forwarder{
			LogInsightReservedFields: []string{"reserved"},
		}
	})

	Describe("Connect", func() {
		It("connect should return true", func() {
			Expect(logger.Connect()).Should(BeTrue())
		})
	})
	Describe("CreateKey", func() {
		It("should return same value", func() {
			Expect(logger.CreateKey("hello")).Should(BeEquivalentTo("hello"))
		})
		It("should return value prefixed with cf_", func() {
			Expect(logger.CreateKey("reserved")).Should(BeEquivalentTo("cf_reserved"))
		})
	})

	Describe("ShipEvents", func() {
		var (
			server *Server
			logger logging.Logging
		)

		BeforeEach(func() {
			server = NewTLSServer()
			url := server.URL()
			port, _ := strconv.Atoi(url[strings.LastIndex(url, ":")+1:])
			logger = loginsight.NewForwarder("127.0.0.1", port, "", "1", false, true, 5)
		})

		AfterEach(func() {
			server.Close()
		})
		It("successfully send messages", func() {
			bodyBytes := []byte(`{"messages":[{"fields":[{"name":"space","content":"test_space"}],"text":"hello","timestamp":10}]}`)
			server.AppendHandlers(
				CombineHandlers(
					VerifyRequest("POST", "/api/v1/messages/ingest/1"),
					VerifyBody(bodyBytes),
					RespondWithJSONEncoded(http.StatusOK, ""),
				),
			)
			fields := make(map[string]interface{})
			fields["timestamp"] = int64(10)
			fields["space"] = "test_space"
			logger.ShipEvents(fields, "hello")
			time.Sleep(2 * time.Second)
			Expect(server.ReceivedRequests()).ShouldNot(BeEmpty())
		})
	})

})
