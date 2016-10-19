package main

import (
	"fmt"
	"log"
	"os"

	"github.com/cloudfoundry-community/firehose-to-syslog/caching"
	"github.com/cloudfoundry-community/firehose-to-syslog/eventRouting"
	"github.com/cloudfoundry-community/firehose-to-syslog/firehoseclient"
	"github.com/cloudfoundry-community/firehose-to-syslog/logging"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/pivotalservices/firehose-to-loginsight/loginsight"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	debug                 = kingpin.Flag("debug", "Enable debug mode. This disables forwarding to syslog").Default("false").OverrideDefaultFromEnvar("DEBUG").Bool()
	apiEndpoint           = kingpin.Flag("api-endpoint", "Api endpoint address. For bosh-lite installation of CF: https://api.10.244.0.34.xip.io").OverrideDefaultFromEnvar("API_ENDPOINT").Required().String()
	dopplerEndpoint       = kingpin.Flag("doppler-endpoint", "Overwrite default doppler endpoint return by /v2/info").OverrideDefaultFromEnvar("DOPPLER_ENDPOINT").String()
	subscriptionID        = kingpin.Flag("subscription-id", "Id for the subscription.").Default("firehose").OverrideDefaultFromEnvar("FIREHOSE_SUBSCRIPTION_ID").String()
	user                  = kingpin.Flag("user", "Admin user.").Default("admin").OverrideDefaultFromEnvar("FIREHOSE_USER").String()
	password              = kingpin.Flag("password", "Admin password.").Default("admin").OverrideDefaultFromEnvar("FIREHOSE_PASSWORD").String()
	skipSSLValidation     = kingpin.Flag("skip-ssl-validation", "Please don't").Default("false").OverrideDefaultFromEnvar("SKIP_SSL_VALIDATION").Bool()
	keepAlive             = kingpin.Flag("fh-keep-alive", "Keep Alive duration for the firehose consumer").Default("25s").OverrideDefaultFromEnvar("FH_KEEP_ALIVE").Duration()
	logEventTotals        = kingpin.Flag("log-event-totals", "Logs the counters for all selected events since nozzle was last started.").Default("false").OverrideDefaultFromEnvar("LOG_EVENT_TOTALS").Bool()
	logEventTotalsTime    = kingpin.Flag("log-event-totals-time", "How frequently the event totals are calculated (in sec).").Default("30s").OverrideDefaultFromEnvar("LOG_EVENT_TOTALS_TIME").Duration()
	wantedEvents          = kingpin.Flag("events", fmt.Sprintf("Comma separated list of events you would like. Valid options are %s", eventRouting.GetListAuthorizedEventEvents())).Default("LogMessage").OverrideDefaultFromEnvar("EVENTS").String()
	boltDatabasePath      = kingpin.Flag("boltdb-path", "Bolt Database path ").Default("my.db").OverrideDefaultFromEnvar("BOLTDB_PATH").String()
	tickerTime            = kingpin.Flag("cc-pull-time", "CloudController Polling time in sec").Default("60s").OverrideDefaultFromEnvar("CF_PULL_TIME").Duration()
	extraFields           = kingpin.Flag("extra-fields", "Extra fields you want to annotate your events with, example: '--extra-fields=env:dev,something:other ").Default("").OverrideDefaultFromEnvar("EXTRA_FIELDS").String()
	logInsightServer      = kingpin.Flag("insight-server", "log insight server address").OverrideDefaultFromEnvar("INSIGHT_SERVER").String()
	logInsightServerPort  = kingpin.Flag("insight-server-port", "log insight server port").OverrideDefaultFromEnvar("INSIGHT_SERVER_PORT").Int()
	logInsightBatchSize   = kingpin.Flag("insight-batch-size", "log insight batch size").Default("1").OverrideDefaultFromEnvar("INSIGHT_BATCH_SIZE").Int()
	logInsightFieldPrefix = kingpin.Flag("insight-field-prefix", "field prefix for log insight tags").Default("cf_").OverrideDefaultFromEnvar("INSIGHT_FIELD_PREFIX").String()
	logInsightAgentID     = kingpin.Flag("insight-agent-id", "agent id for log insight").Default("1").OverrideDefaultFromEnvar("INSIGHT_AGENT_ID").String()
)

var (
	VERSION = "0.0.0"
)

func main() {
	kingpin.Version(VERSION)
	kingpin.Parse()

	var loggingClient logging.Logging
	//Setup Logging
	loggingClient = loginsight.NewLogging(logInsightServer, logInsightServerPort, logInsightBatchSize, logInsightFieldPrefix, logInsightAgentID)
	logging.LogStd(fmt.Sprintf("Starting firehose-to-loginsight %s ", VERSION), true)

	c := cfclient.Config{
		ApiAddress:        *apiEndpoint,
		Username:          *user,
		Password:          *password,
		SkipSslValidation: *skipSSLValidation,
	}
	cloudFoundryClient := cfclient.NewClient(&c)

	if len(*dopplerEndpoint) > 0 {
		cloudFoundryClient.Endpoint.DopplerEndpoint = *dopplerEndpoint
	}
	logging.LogStd(fmt.Sprintf("Using %s as doppler endpoint", cloudFoundryClient.Endpoint.DopplerEndpoint), true)

	//Creating Caching
	var cachingClient caching.Caching
	if caching.IsNeeded(*wantedEvents) {
		cachingClient = caching.NewCachingBolt(cloudFoundryClient, *boltDatabasePath)
	} else {
		cachingClient = caching.NewCachingEmpty()
	}
	//Creating Events
	events := eventRouting.NewEventRouting(cachingClient, loggingClient)
	err := events.SetupEventRouting(*wantedEvents)
	if err != nil {
		log.Fatal("Error setting up event routing: ", err)
		os.Exit(1)

	}

	//Set extrafields if needed
	events.SetExtraFields(*extraFields)

	//Enable LogsTotalevent
	if *logEventTotals {
		logging.LogStd("Logging total events %", true)
		events.LogEventTotals(*logEventTotalsTime)
	}

	// Parse extra fields from cmd call
	cachingClient.CreateBucket()
	//Let's Update the database the first time
	logging.LogStd("Start filling app/space/org cache.", true)
	apps := cachingClient.GetAllApp()
	logging.LogStd(fmt.Sprintf("Done filling cache! Found [%d] Apps", len(apps)), true)

	//Let's start the goRoutine
	cachingClient.PerformPoollingCaching(*tickerTime)

	firehoseConfig := &firehoseclient.FirehoseConfig{
		TrafficControllerURL:   *dopplerEndpoint,
		InsecureSSLSkipVerify:  *skipSSLValidation,
		IdleTimeoutSeconds:     *keepAlive,
		FirehoseSubscriptionID: *subscriptionID,
	}

	if loggingClient.Connect() || *debug {

		logging.LogStd("Connecting to Firehose...", true)
		firehoseClient := firehoseclient.NewFirehoseNozzle(cloudFoundryClient, events, firehoseConfig)
		err = firehoseClient.Start()
		if err != nil {
			logging.LogError("Failed connecting to Firehose...Please check settings and try again!", "")

		} else {
			logging.LogStd("Firehose Subscription Succesfull! Routing events...", true)
		}

	} else {
		logging.LogError("Failed connecting Log Insight...Please check settings and try again!", "")
	}

	defer cachingClient.Close()
}
