package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Shopify/sarama"
	kingpin "github.com/alecthomas/kingpin/v2"
	"github.com/danielqsj/kafka_exporter/exporter"
	"github.com/prometheus/client_golang/prometheus"
	plog "github.com/prometheus/common/promlog"
	plogflag "github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/rcrowley/go-metrics"
)

const (
	namespace = "kafka"
	clientID  = "kafka_exporter"
)

// CanReadCertAndKey returns true if the certificate and key files already exists,
// otherwise returns false. If lost one of cert and key, returns error.
func CanReadCertAndKey(certPath, keyPath string) (bool, error) {
	certReadable := canReadFile(certPath)
	keyReadable := canReadFile(keyPath)

	if certReadable == false && keyReadable == false {
		return false, nil
	}

	if certReadable == false {
		return false, fmt.Errorf("error reading %s, certificate and key must be supplied as a pair", certPath)
	}

	if keyReadable == false {
		return false, fmt.Errorf("error reading %s, certificate and key must be supplied as a pair", keyPath)
	}

	return true, nil
}

// If the file represented by path exists and
// readable, returns true otherwise returns false.
func canReadFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}

	defer f.Close()

	return true
}

func init() {
	metrics.UseNilMetrics = true
	prometheus.MustRegister(version.NewCollector("kafka_exporter"))
}

//func toFlag(name string, help string) *kingpin.FlagClause {
//	flag.CommandLine.String(name, "", help) // hack around flag.Parse and klog.init flags
//	return kingpin.Flag(name, help)
//}

// hack around flag.Parse and klog.init flags
func toFlagString(name string, help string, value string) *string {
	flag.CommandLine.String(name, value, help) // hack around flag.Parse and klog.init flags
	return kingpin.Flag(name, help).Default(value).String()
}

func toFlagBool(name string, help string, value bool, valueString string) *bool {
	flag.CommandLine.Bool(name, value, help) // hack around flag.Parse and klog.init flags
	return kingpin.Flag(name, help).Default(valueString).Bool()
}

func toFlagStringsVar(name string, help string, value string, target *[]string) {
	flag.CommandLine.String(name, value, help) // hack around flag.Parse and klog.init flags
	kingpin.Flag(name, help).Default(value).StringsVar(target)
}

func toFlagStringVar(name string, help string, value string, target *string) {
	flag.CommandLine.String(name, value, help) // hack around flag.Parse and klog.init flags
	kingpin.Flag(name, help).Default(value).StringVar(target)
}

func toFlagBoolVar(name string, help string, value bool, valueString string, target *bool) {
	flag.CommandLine.Bool(name, value, help) // hack around flag.Parse and klog.init flags
	kingpin.Flag(name, help).Default(valueString).BoolVar(target)
}

func toFlagIntVar(name string, help string, value int, valueString string, target *int) {
	flag.CommandLine.Int(name, value, help) // hack around flag.Parse and klog.init flags
	kingpin.Flag(name, help).Default(valueString).IntVar(target)
}

func main() {
	var (
		listenAddress = toFlagString("web.listen-address", "Address to listen on for web interface and telemetry.", ":9308")
		metricsPath   = toFlagString("web.telemetry-path", "Path under which to expose metrics.", "/metrics")
		topicFilter   = toFlagString("topic.filter", "Regex that determines which topics to collect.", ".*")
		topicExclude  = toFlagString("topic.exclude", "Regex that determines which topics to exclude.", "^$")
		groupFilter   = toFlagString("group.filter", "Regex that determines which consumer groups to collect.", ".*")
		groupExclude  = toFlagString("group.exclude", "Regex that determines which consumer groups to exclude.", "^$")
		logSarama     = toFlagBool("log.enable-sarama", "Turn on Sarama logging, default is false.", false, "false")

		opts = exporter.Options{}
	)

	toFlagStringsVar("kafka.server", "Address (host:port) of Kafka server.", "kafka:9092", &opts.Uri)
	toFlagBoolVar("sasl.enabled", "Connect using SASL/PLAIN, default is false.", false, "false", &opts.UseSASL)
	toFlagBoolVar("sasl.handshake", "Only set this to false if using a non-Kafka SASL proxy, default is true.", true, "true", &opts.UseSASLHandshake)
	toFlagStringVar("sasl.username", "SASL user name.", "", &opts.SaslUsername)
	toFlagStringVar("sasl.password", "SASL user password.", "", &opts.SaslPassword)
	toFlagStringVar("sasl.mechanism", "The SASL SCRAM SHA algorithm sha256 or sha512 or gssapi as mechanism", "", &opts.SaslMechanism)
	toFlagStringVar("sasl.service-name", "Service name when using kerberos Auth", "", &opts.ServiceName)
	toFlagStringVar("sasl.kerberos-config-path", "Kerberos config path", "", &opts.KerberosConfigPath)
	toFlagStringVar("sasl.realm", "Kerberos realm", "", &opts.Realm)
	toFlagStringVar("sasl.kerberos-auth-type", "Kerberos auth type. Either 'keytabAuth' or 'userAuth'", "", &opts.KerberosAuthType)
	toFlagStringVar("sasl.keytab-path", "Kerberos keytab file path", "", &opts.KeyTabPath)
	toFlagBoolVar("sasl.disable-PA-FX-FAST", "Configure the Kerberos client to not use PA_FX_FAST, default is false.", false, "false", &opts.saslDisablePAFXFast)
	toFlagBoolVar("tls.enabled", "Connect to Kafka using TLS, default is false.", false, "false", &opts.UseTLS)
	toFlagStringVar("tls.server-name", "Used to verify the hostname on the returned certificates unless tls.insecure-skip-tls-verify is given. The kafka server's name should be given.", "", &opts.tlsServerName)
	toFlagStringVar("tls.ca-file", "The optional certificate authority file for Kafka TLS client authentication.", "", &opts.TlsCAFile)
	toFlagStringVar("tls.cert-file", "The optional certificate file for Kafka client authentication.", "", &opts.TlsCertFile)
	toFlagStringVar("tls.key-file", "The optional key file for Kafka client authentication.", "", &opts.TlsKeyFile)
	toFlagBoolVar("server.tls.enabled", "Enable TLS for web server, default is false.", false, "false", &opts.serverUseTLS)
	toFlagBoolVar("server.tls.mutual-auth-enabled", "Enable TLS client mutual authentication, default is false.", false, "false", &opts.serverMutualAuthEnabled)
	toFlagStringVar("server.tls.ca-file", "The certificate authority file for the web server.", "", &opts.ServerTlsCAFile)
	toFlagStringVar("server.tls.cert-file", "The certificate file for the web server.", "", &opts.ServerTlsCertFile)
	toFlagStringVar("server.tls.key-file", "The key file for the web server.", "", &opts.ServerTlsKeyFile)
	toFlagBoolVar("tls.insecure-skip-tls-verify", "If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure. Default is false", false, "false", &opts.TlsInsecureSkipTLSVerify)
	toFlagStringVar("kafka.version", "Kafka broker version", sarama.V2_0_0_0.String(), &opts.KafkaVersion)
	toFlagBoolVar("use.consumelag.zookeeper", "if you need to use a group from zookeeper, default is false", false, "false", &opts.UseZooKeeperLag)
	toFlagStringsVar("zookeeper.server", "Address (hosts) of zookeeper server.", "localhost:2181", &opts.UriZookeeper)
	toFlagStringVar("kafka.labels", "Kafka cluster name", "", &opts.Labels)
	toFlagStringVar("refresh.metadata", "Metadata refresh interval", "30s", &opts.metadataRefreshInterval)
	toFlagBoolVar("offset.show-all", "Whether show the offset/lag for all consumer group, otherwise, only show connected consumer groups, default is true", true, "true", &opts.OffsetShowAll)
	toFlagBoolVar("concurrent.enable", "If true, all scrapes will trigger kafka operations otherwise, they will share results. WARN: This should be disabled on large clusters. Default is false", false, "false", &opts.allowConcurrent)
	toFlagIntVar("topic.workers", "Number of topic workers", 100, "100", &opts.topicWorkers)
	toFlagBoolVar("kafka.allow-auto-topic-creation", "If true, the broker may auto-create topics that we requested which do not already exist, default is false.", false, "false", &opts.allowAutoTopicCreation)
	toFlagIntVar("verbosity", "Verbosity log level", 0, "0", &opts.verbosityLogLevel)

	plConfig := plog.Config{}
	plogflag.AddFlags(kingpin.CommandLine, &plConfig)
	kingpin.Version(version.Print("kafka_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	labels := make(map[string]string)

	// Protect against empty labels
	if opts.labels != "" {
		for _, label := range strings.Split(opts.labels, ",") {
			splitted := strings.Split(label, "=")
			if len(splitted) >= 2 {
				labels[splitted[0]] = splitted[1]
			}
		}
	}

	setup(*listenAddress, *metricsPath, *topicFilter, *topicExclude, *groupFilter, *groupExclude, *logSarama, opts, labels)
}
