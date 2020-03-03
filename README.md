#Disclaimer

Since v0.0.10 we stop supporting username and password for authentication.

Please use ClientId and ClientSecret.

# firehose-to-loginsight
Firehose nozzle to pull events and send to LogInsight ingestion API inspired and leverages firehose-to-sylog

# Options

```bash
usage: main --api-endpoint=API-ENDPOINT [<flags>]

Flags:
  --help                       Show context-sensitive help (also try --help-long and --help-man).
  --debug                      Enable debug mode. This enables additional logging
  --api-endpoint=API-ENDPOINT  Api endpoint address. For bosh-lite installation of CF: https://api.10.244.0.34.xip.io
  --doppler-endpoint=DOPPLER-ENDPOINT  
                               Overwrite default doppler endpoint return by /v2/info
  --subscription-id="firehose-to-loginsight"  
                               Id for the subscription.
  --client-id="admin"          Client ID.
  --client-secret="admin-client-secret"  
                               Client Secret.
  --skip-ssl-validation        Please don't
  --fh-keep-alive=25s          Keep Alive duration for the firehose consumer
  --events="LogMessage"        Comma separated list of events you would like. Valid options are ContainerMetric, CounterEvent, Error, HttpStartStop, LogMessage,
                               ValueMetric
  --boltdb-path="my.db"        Bolt Database path
  --cc-pull-time=60s           CloudController Polling time in sec
  --extra-fields=""            Extra fields you want to annotate your events with, example: '--extra-fields=env:dev,something:other
  --insight-server=INSIGHT-SERVER  
                               log insight server address
  --insight-server-port=9543   log insight server port
  --insight-reserved-fields="event_type"  
                               comma delimited list of fields that are reserved
  --insight-agent-id="1"       agent id for log insight
  --insight-has-json-log-msg   app log message can be json
  --concurrent-workers=50      number of concurrent workers pulling messages from channel
  --noop                       if it should avoid sending to log-insight
  --max-idle-connections=100   Max http idle connections
  --max-idle-connections-per-host=100  
                               max idle connections per host
  --idle-connection-timeout-seconds=90  
                               seconds for timeout
  --min-retry-delay=500ms      Doppler Cloud Foundry Doppler min. retry delay duration
  --max-retry-delay=1m         Doppler Cloud Foundry Doppler max. retry delay duration
  --max-retry-count=1000       Doppler Cloud Foundry Doppler max. retry Count duration
  --logs-buffer-size=10000     Number of envelope to be buffered
  --enable-stats-server        Will enable stats server on 8080
  --cc-rps=50                  CloudController Polling request by second
  --orgs=""                    Forwarded on the app logs from theses organisations' example: --orgs=org1,org2
  --ignore-missing-apps        Enable throttling on cache lookup for missing apps
  --version                    Show application version.
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
uaac client add firehose-to-loginsight \
      --name firehose-to-loginsight \
      --secret [your_client_secret] \
      --authorized_grant_types client_credentials,refresh_token \
      --authorities doppler.firehose,cloud_controller.admin_read_only
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
cf set-env firehose-to-loginsight INSIGHT_SERVER [Your Log Insight IP]
cf set-env firehose-to-loginsight INSIGHT_SERVER_PORT [Your Log Insight Ingestion Port, defaults to 9543]
cf set-env firehose-to-loginsight LOG_EVENT_TOTALS true
cf set-env firehose-to-loginsight LOG_EVENT_TOTALS_TIME "10s"
cf set-env firehose-to-loginsight SKIP_SSL_VALIDATION true
cf set-env firehose-to-loginsight FIREHOSE_SUBSCRIPTION_ID firehose-to-loginsight
cf set-env firehose-to-loginsight FIREHOSE_CLIENT_ID  [your doppler.firehose enabled user]
cf set-env firehose-to-loginsight FIREHOSE_CLIENT_SECRET  [your doppler.firehose enabled user password]
```

## Push the app.

```bash
cf push firehose-to-loginsight --no-route
```

# Contributors

* [Caleb Washburn](https://github.com/calebwashburn)
* [Ian Zink](https://github.com/z4ce)
