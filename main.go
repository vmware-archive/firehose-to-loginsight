package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/cloudfoundry-community/firehose-to-syslog/caching"
	"github.com/cloudfoundry-community/firehose-to-syslog/eventRouting"
	"github.com/cloudfoundry-community/firehose-to-syslog/firehoseclient"
	"github.com/cloudfoundry-community/firehose-to-syslog/logging"
	"github.com/cloudfoundry-community/firehose-to-syslog/stats"
	"github.com/cloudfoundry-community/firehose-to-syslog/uaatokenrefresher"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/pivotalservices/firehose-to-loginsight/loginsight"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	debug                    = kingpin.Flag("debug", "Enable debug mode. This enables additional logging").Default("false").OverrideDefaultFromEnvar("DEBUG").Bool()
	apiEndpoint              = kingpin.Flag("api-endpoint", "Api endpoint address. For bosh-lite installation of CF: https://api.10.244.0.34.xip.io").OverrideDefaultFromEnvar("API_ENDPOINT").Required().String()
	dopplerEndpoint          = kingpin.Flag("doppler-endpoint", "Overwrite default doppler endpoint return by /v2/info").OverrideDefaultFromEnvar("DOPPLER_ENDPOINT").String()
	subscriptionID           = kingpin.Flag("subscription-id", "Id for the subscription.").Default("firehose-to-loginsight").OverrideDefaultFromEnvar("FIREHOSE_SUBSCRIPTION_ID").String()
	clientID                 = kingpin.Flag("client-id", "Client ID.").Default("admin").OverrideDefaultFromEnvar("FIREHOSE_CLIENT_ID").String()
	clientSecret             = kingpin.Flag("client-secret", "Client Secret.").Default("admin-client-secret").OverrideDefaultFromEnvar("FIREHOSE_CLIENT_SECRET").String()
	skipSSLValidation        = kingpin.Flag("skip-ssl-validation", "Please don't").Default("false").OverrideDefaultFromEnvar("SKIP_SSL_VALIDATION").Bool()
	keepAlive                = kingpin.Flag("fh-keep-alive", "Keep Alive duration for the firehose consumer").Default("25s").OverrideDefaultFromEnvar("FH_KEEP_ALIVE").Duration()
	wantedEvents             = kingpin.Flag("events", fmt.Sprintf("Comma separated list of events you would like. Valid options are %s", eventRouting.GetListAuthorizedEventEvents())).Default("LogMessage").OverrideDefaultFromEnvar("EVENTS").String()
	boltDatabasePath         = kingpin.Flag("boltdb-path", "Bolt Database path ").Default("my.db").OverrideDefaultFromEnvar("BOLTDB_PATH").String()
	tickerTime               = kingpin.Flag("cc-pull-time", "CloudController Polling time in sec").Default("60s").OverrideDefaultFromEnvar("CF_PULL_TIME").Duration()
	extraFields              = kingpin.Flag("extra-fields", "Extra fields you want to annotate your events with, example: '--extra-fields=env:dev,something:other ").Default("").OverrideDefaultFromEnvar("EXTRA_FIELDS").String()
	logInsightServer         = kingpin.Flag("insight-server", "log insight server address").OverrideDefaultFromEnvar("INSIGHT_SERVER").String()
	logInsightServerPort     = kingpin.Flag("insight-server-port", "log insight server port").Default("9543").OverrideDefaultFromEnvar("INSIGHT_SERVER_PORT").Int()
	logInsightReservedFields = kingpin.Flag("insight-reserved-fields", "comma delimited list of fields that are reserved").Default("event_type").OverrideDefaultFromEnvar("INSIGHT_RESERVED_FIELDS").String()
	logInsightAgentID        = kingpin.Flag("insight-agent-id", "agent id for log insight").Default("1").OverrideDefaultFromEnvar("INSIGHT_AGENT_ID").String()
	logInsightHasJSONLogMsg  = kingpin.Flag("insight-has-json-log-msg", "app log message can be json").Default("false").OverrideDefaultFromEnvar("INSIGHT_HAS_JSON_LOG_MSG").Bool()
	concurrentWorkers        = kingpin.Flag("concurrent-workers", "number of concurrent workers pulling messages from channel").Default("50").OverrideDefaultFromEnvar("CONCURRENT_WORKERS").Int()
	noop                     = kingpin.Flag("noop", "if it should avoid sending to log-insight").Default("false").OverrideDefaultFromEnvar("INSIGHT_NOOP").Bool()
	minRetryDelay            = kingpin.Flag("min-retry-delay", "Doppler Cloud Foundry Doppler min. retry delay duration").Default("500ms").Envar("MIN_RETRY_DELAY").Duration()
	maxRetryDelay            = kingpin.Flag("max-retry-delay", "Doppler Cloud Foundry Doppler max. retry delay duration").Default("1m").Envar("MAX_RETRY_DELAY").Duration()
	maxRetryCount            = kingpin.Flag("max-retry-count", "Doppler Cloud Foundry Doppler max. retry Count duration").Default("1000").Envar("MAX_RETRY_COUNT").Int()
	bufferSize               = kingpin.Flag("logs-buffer-size", "Number of envelope to be buffered").Default("10000").Envar("LOGS_BUFFER_SIZE").Int()
	statServer               = kingpin.Flag("enable-stats-server", "Will enable stats server on 8080").Default("false").Envar("ENABLE_STATS_SERVER").Bool()
	requestLimit             = kingpin.Flag("cc-rps", "CloudController Polling request by second").Default("50").Envar("CF_RPS").Int()
	orgs                     = kingpin.Flag("orgs", "Forwarded on the app logs from theses organisations' example: --orgs=org1,org2").Default("").Envar("ORGS").String()
	ignoreMissingApps        = kingpin.Flag("ignore-missing-apps", "Enable throttling on cache lookup for missing apps").Envar("IGNORE_MISSING_APPS").Default("false").Bool()
)

var (
	VERSION = "0.0.0"
)

func main() {
	kingpin.Version(VERSION)
	kingpin.Parse()

	var loggingClient logging.Logging
	//Setup Logging
	logging.LogStd(fmt.Sprintf("Starting firehose-to-loginsight %s ", VERSION), true)
	if len(*apiEndpoint) <= 0 {
		log.Fatal("Must set api-endpoint property")
		os.Exit(1)
	}
	if !*noop {
		if len(*logInsightServer) <= 0 {
			log.Fatal("Must set insight-server property")
			os.Exit(1)
		}
		loggingClient = loginsight.NewForwarder(*logInsightServer, *logInsightServerPort, *logInsightReservedFields, *logInsightAgentID, *logInsightHasJSONLogMsg, *debug, *concurrentWorkers, *skipSSLValidation)
	} else {
		loggingClient = loginsight.NewNoopForwarder()
	}

	c := cfclient.Config{
		ApiAddress:        *apiEndpoint,
		ClientID:          *clientID,
		ClientSecret:      *clientSecret,
		SkipSslValidation: *skipSSLValidation,
		UserAgent:         "firehose-to-loginsight/" + VERSION,
	}
	cfClient, err := cfclient.NewClient(&c)
	if err != nil {
		log.Fatal("New Client: ", err)
		os.Exit(1)
	}
	if len(*dopplerEndpoint) > 0 {
		cfClient.Endpoint.DopplerEndpoint = *dopplerEndpoint
	}
	fmt.Println(cfClient.Endpoint.DopplerEndpoint)
	logging.LogStd(fmt.Sprintf("Using %s as doppler endpoint", cfClient.Endpoint.DopplerEndpoint), true)

	//Creating Caching
	var cachingClient caching.Caching
	if caching.IsNeeded(*wantedEvents) {
		config := &caching.CachingBoltConfig{
			Path:               *boltDatabasePath,
			IgnoreMissingApps:  *ignoreMissingApps,
			CacheInvalidateTTL: *tickerTime,
			RequestBySec:       *requestLimit,
		}
		cachingClient, err = caching.NewCachingBolt(cfClient, config)

		if err != nil {
			log.Fatal("Failed to create boltdb cache: ", err)
			os.Exit(1)
		}
	} else {
		cachingClient = caching.NewCachingEmpty()
	}

	if err := cachingClient.Open(); err != nil {
		log.Fatal("Error open cache: ", err)
		os.Exit(1)
	}
	defer cachingClient.Close()

	//Adding Stats
	statistic := stats.NewStats()
	go statistic.PerSec()

	////Starting Http Server for Stats
	if *statServer {
		Server := &stats.Server{
			Logger: log.New(os.Stdout, "", 1),
			Stats:  statistic,
		}

		go Server.Start()
	}

	//Creating Events
	eventFilters := []eventRouting.EventFilter{eventRouting.HasIgnoreField, eventRouting.NotInCertainOrgs(*orgs)}
	events := eventRouting.NewEventRouting(cachingClient, loggingClient, statistic, eventFilters)
	err = events.SetupEventRouting(*wantedEvents)
	if err != nil {
		log.Fatal("Error setting up event routing: ", err)
		os.Exit(1)
	}

	//Set extrafields if needed
	events.SetExtraFields(*extraFields)

	uaaRefresher, err := uaatokenrefresher.NewUAATokenRefresher(
		cfClient.Endpoint.AuthEndpoint,
		*clientID,
		*clientSecret,
		*skipSSLValidation,
	)

	if err != nil {
		logging.LogError(fmt.Sprint("Failed connecting to Get token from UAA..", err), "")
	}

	firehoseConfig := &firehoseclient.FirehoseConfig{
		TrafficControllerURL:   cfClient.Endpoint.DopplerEndpoint,
		InsecureSSLSkipVerify:  *skipSSLValidation,
		IdleTimeoutSeconds:     *keepAlive,
		FirehoseSubscriptionID: *subscriptionID,
		MinRetryDelay:          *minRetryDelay,
		MaxRetryDelay:          *maxRetryDelay,
		MaxRetryCount:          *maxRetryCount,
		BufferSize:             *bufferSize,
	}

	if loggingClient.Connect() || *debug {

		logging.LogStd("Connecting to Firehose...", true)
		firehoseClient := firehoseclient.NewFirehoseNozzle(uaaRefresher, events, firehoseConfig, statistic)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		firehoseClient.Start(ctx)

		signalChan := make(chan os.Signal, 1)
		cleanupDone := make(chan bool)
		signal.Notify(signalChan, os.Interrupt, os.Kill)
		go func() {
			for range signalChan {
				fmt.Println("\nSignal Received, Stop reading and starting Draining...")
				firehoseClient.StopReading()
				cctx, tcancel := context.WithTimeout(context.TODO(), 30*time.Second)
				tcancel()
				firehoseClient.Draining(cctx)
				cleanupDone <- true
			}
		}()
		<-cleanupDone
	} else {
		logging.LogError("Failed connecting Log Insight...Please check settings and try again!", "")
		os.Exit(1)
	}
}
