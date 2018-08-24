package etcdutil

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"os"
	"strings"
	"time"

	etcd "github.com/coreos/etcd/clientv3"
	"github.com/mailgun/holster"
	"github.com/pkg/errors"
	"google.golang.org/grpc/grpclog"
)

const (
	pathToCA          = "/etc/mailgun/ssl/localhost/ca.pem"
	pathToKey         = "/etc/mailgun/ssl/localhost/etcd-key.pem"
	pathToCert        = "/etc/mailgun/ssl/localhost/etcd-cert.pem"
	localEtcdEndpoint = "127.0.0.1:2379"
)

func init() {
	// We check this here to avoid data race with GRPC go routines writing to the logger
	if os.Getenv("ETCD3_DEBUG") != "" {
		etcd.SetLogger(grpclog.NewLoggerV2WithVerbosity(os.Stderr, os.Stderr, os.Stderr, 4))
	}
}

func NewSecureClient(cfg *etcd.Config) (*etcd.Client, error) {
	var err error
	if cfg, err = NewEtcdConfig(cfg); err != nil {
		return nil, errors.Wrap(err, "failed to build etcd config")
	}

	etcdClt, err := etcd.New(*cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create secure etcd client")
	}
	return etcdClt, nil
}

// Create a new etcd.Config using environment variables. If an existing
// config is passed, will fill in missing configuration using environment
// variables or defaults if they exists on the local system.

// If no environment variables are set, will return a config set to
// connect without TLS via localhost:2379
func NewEtcdConfig(cfg *etcd.Config) (*etcd.Config, error) {
	var envEndpoint, tlsCertFile, tlsKeyFile, tlsCaFile string

	// Create a config if none exists and get user/pass
	holster.SetDefault(&cfg, &etcd.Config{})
	holster.SetDefault(&cfg.Username, os.Getenv("ETCD3_USER"))
	holster.SetDefault(&cfg.Password, os.Getenv("ETCD3_PASSWORD"))

	// Don't set default file locations for these if they don't exist on disk
	// as dev or testing environments might not have certificates
	holster.SetDefault(&tlsCertFile, os.Getenv("ETCD3_TLS_CERT"), ifExists(pathToCert))
	holster.SetDefault(&tlsKeyFile, os.Getenv("ETCD3_TLS_KEY"), ifExists(pathToKey))
	holster.SetDefault(&tlsCaFile, os.Getenv("ETCD3_CA"), ifExists(pathToCA))

	// Default to 5 second timeout, else connections hang indefinitely
	holster.SetDefault(&cfg.DialTimeout, time.Second*5)
	// Or if the user provided a timeout
	if timeout := os.Getenv("ETCD3_DIAL_TIMEOUT"); timeout != "" {
		duration, err := time.ParseDuration(timeout)
		if err != nil {
			return nil, errors.Errorf(
				"ETCD3_DIAL_TIMEOUT='%s' is not a duration (1m|15s|24h): %s", timeout, err)
		}
		cfg.DialTimeout = duration
	}

	// If the CA file was provided
	if tlsCaFile != "" {
		holster.SetDefault(&cfg.TLS, &tls.Config{})

		var certPool *x509.CertPool = nil
		if pemBytes, err := ioutil.ReadFile(tlsCaFile); err == nil {
			certPool = x509.NewCertPool()
			certPool.AppendCertsFromPEM(pemBytes)
		} else {
			return nil, errors.Errorf("while loading cert CA file '%s': %s", tlsCaFile, err)
		}
		holster.SetDefault(&cfg.TLS.RootCAs, certPool)
		cfg.TLS.InsecureSkipVerify = false
	}

	// If the cert and key files are provided attempt to load them
	if tlsCertFile != "" && tlsKeyFile != "" {
		holster.SetDefault(&cfg.TLS, &tls.Config{})
		tlsCert, err := tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile)
		if err != nil {
			return nil, errors.Errorf("while loading cert '%s' and key file '%s': %s",
				tlsCertFile, tlsKeyFile, err)
		}
		holster.SetDefault(&cfg.TLS.Certificates, []tls.Certificate{tlsCert})
	}

	holster.SetDefault(&envEndpoint, os.Getenv("ETCD3_ENDPOINT"), localEtcdEndpoint)
	holster.SetDefault(&cfg.Endpoints, strings.Split(envEndpoint, ","))

	// If no other TLS config is provided this will force connecting with TLS,
	// without cert verification
	if os.Getenv("ETCD3_SKIP_VERIFY") != "" {
		holster.SetDefault(&cfg.TLS, &tls.Config{})
		cfg.TLS.InsecureSkipVerify = true
	}

	// Enable TLS with no additional configuration
	if os.Getenv("ETCD3_ENABLE_TLS") != "" {
		holster.SetDefault(&cfg.TLS, &tls.Config{})
	}

	return cfg, nil
}

// If the file exists, return the path provided
func ifExists(file string) string {
	if _, err := os.Stat(file); err == nil {
		return file
	}
	return ""
}
