# firehose-to-loginsight
Firehose nozzle to pull events and send to LogInsight ingestion API inspired and leverages firehose-to-sylog

# Options

```bash
usage: firehose-to-loginsight --api-endpoint=API-ENDPOINT [<flags>]

Flags:
  --help                         Show context-sensitive help (also try --help-long and --help-man).
  --debug                        Enable debug mode. This disables forwarding to syslog
  --api-endpoint=API-ENDPOINT    Api endpoint address. For bosh-lite installation of CF: https://api.10.244.0.34.xip.io
  --doppler-endpoint=DOPPLER-ENDPOINT
  --insight-server=INSIGHT_SERVER
                                 log insight server address
  --insight-server-port=INSIGHT_SERVER_PORT
                                 log insight server port
  --insight-batch-size=INSIGHT_BATCH_SIZE
                                 log insight batch size
  --insight-field-prefix=INSIGHT_FIELD_PREFIX
                                 field prefix for log insight
  --subscription-id="firehose"   Id for the subscription.
  --user="admin"                 Admin user.
  --password="admin"             Admin password.
  --skip-ssl-validation          Please don't
  --fh-keep-alive=25s            Keep Alive duration for the firehose consumer
  --log-event-totals             Logs the counters for all selected events since nozzle was last started.
  --log-event-totals-time=30s    How frequently the event totals are calculated (in sec).
  --events="LogMessage"          Comma separated list of events you would like. Valid options are Error, ContainerMetric,
                                 HttpStart, HttpStop, HttpStartStop, LogMessage, ValueMetric, CounterEvent
  --boltdb-path="my.db"          Bolt Database path
  --cc-pull-time=60s             CloudController Polling time in sec
  --extra-fields=""              Extra fields you want to annotate your events with, example:
                                 '--extra-fields=env:dev,something:other
  --version                      Show application version.
```

** !!! **--events** Please use --help to get last updated event.


#Endpoint definition

We use [gocf-client](https://github.com/cloudfoundry-community/go-cfclient) which will call the CF endpoint /v2/info to get Auth., doppler endpoint.

But for doppler endpoint you can overwrite it with ``` --doppler-address ``` as we know some people use different endpoint.

# Event documentation

See the [dropsonde protocol documentation](https://github.com/cloudfoundry/dropsonde-protocol/tree/master/events) for details on what data is sent as part of each event.

# Caching
We use [boltdb](https://github.com/boltdb/bolt) for caching application name, org and space name.

We have 3 caching strategies:
* Pull all application data on start.
* Pull application data if not cached yet.
* Pull all application data every "cc-pull-time".

# To test and build

```bash
    # Setup repo
    go get github.com/pivotalservices/firehose-to-loginsight
    cd $GOPATH/src/github.com/pivotalservices/firehose-to-loginsight
    glide install --strip-vendor —strip-vcs
    # Test
	ginkgo -r .

    # Build binary
    go build
```
# Run against a bosh-lite CF deployment
```bash
    go run main.go \
		--debug \
		--skip-ssl-validation \
		--api-endpoint="https://api.10.244.0.34.xip.io"
```

# Parsing the logs with Logstash

[logsearch-for-cloudfoundry](https://github.com/logsearch/logsearch-for-cloudfoundry)

# Push as an App to Cloud Foundry

## Create doppler.firehose enabled user

```bash
uaac target https://uaa.[your cf system domain] --skip-ssl-validation
uaac token client get admin -s [your admin-secret]
cf create-user [firehose user] [firehose password]
uaac member add cloud_controller.admin [your firehose user]
uaac member add doppler.firehose [your firehose user]
```

## Download the latest release of firehose-to-loginsight from GITHub releases (https://github.com/pivotalservices/firehose-to-loginsight/releases)

```bash
chmod +x firehose-to-loginsight
```

## Utilize the CF cli to authenticate with your PCF instance.

```bash
cf login -a https://api.[your cf system domain] -u [your id] --skip-ssl-validation
```

## Push firehose-to-loginsight.
```bash
cf push firehose-to-loginsight -c ./firehose-to-loginsight -b binary_buildpack -u none --no-start
```

## Set environment variables with cf cli or in the [manifest.yml](./manifest.yml).

```bash
cf set-env firehose-to-loginsight API_ENDPOINT https://api.[your cf system domain]
cf set-env firehose-to-loginsight DOPPLER_ENDPOINT wss://doppler.[your cf system domain]:443
cf set-env firehose-to-loginsight INSIGHT_SERVER [Your Log Insight IP]
cf set-env firehose-to-loginsight INSIGHT_SERVER_PORT [Your Log Insight Ingestion Port]
cf set-env firehose-to-loginsight INSIGHT_BATCH_SIZE [Batch size, default 50]
cf set-env firehose-to-loginsight INSIGHT_FIELD_PREFIX [Prefix for fields, default 'cf_']
cf set-env firehose-to-loginsight LOG_EVENT_TOTALS true
cf set-env firehose-to-loginsight LOG_EVENT_TOTALS_TIME "10s"
cf set-env firehose-to-loginsight SKIP_SSL_VALIDATION true
cf set-env firehose-to-loginsight FIREHOSE_SUBSCRIPTION_ID firehose-to-loginsight
cf set-env firehose-to-loginsight FIREHOSE_USER  [your doppler.firehose enabled user]
cf set-env firehose-to-loginsight FIREHOSE_PASSWORD  [your doppler.firehose enabled user password]
```

## Push the app.

```bash
cf push firehose-to-loginsight --no-route
```

# Contributors

* [Caleb Washburn](https://github.com/calebwashburn)
* [Ian Zink](https://github.com/z4ce)
