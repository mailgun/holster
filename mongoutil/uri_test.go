package mongoutil_test

import (
	"encoding/json"
	"testing"

	"github.com/mailgun/holster/v4/mongoutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestMongoConfig_URIWithOptions(t *testing.T) {
	for _, tt := range []struct {
		cfg  mongoutil.Config
		name string
		uri  string
	}{{
		name: "Bare minimum config",
		cfg: mongoutil.Config{
			URI: "mongodb://127.0.0.1:27017/foo",
		},
		uri: "mongodb://127.0.0.1:27017/foo",
	}, {
		name: "URI parameters appended after URI encoded parameters",
		cfg: mongoutil.Config{
			URI: "mongodb://127.0.0.1:27017/foo?replicaSet=bazz&blah=wow",
			Options: []map[string]interface{}{
				{"replica_set": "bar"},
				{"max_pool_size": 5},
			},
		},
		uri: "mongodb://127.0.0.1:27017/foo?replicaSet=bazz&blah=wow&replicaSet=bar&maxPoolSize=5",
	}, {
		name: "Read preference provided",
		cfg: mongoutil.Config{
			URI: "mongodb://127.0.0.1:27017/foo",
			Options: []map[string]interface{}{
				{"read_preference": "secondaryPreferred"},
			},
		},
		uri: "mongodb://127.0.0.1:27017/foo?readPreference=secondaryPreferred",
	}, {
		name: "Servers and Database provided",
		cfg: mongoutil.Config{
			Servers: []string{
				"mongodb-n01:27017",
				"mongodb-n02:28017",
			},
			Database: "foo",
			Options: []map[string]interface{}{
				{"read_preference": "secondaryPreferred"},
			},
		},
		uri: "mongodb://mongodb-n01:27017,mongodb-n02:28017/foo?readPreference=secondaryPreferred",
	}} {
		t.Run(tt.name, func(t *testing.T) {
			uri := tt.cfg.URIWithOptions()
			assert.Equal(t, tt.uri, uri)
		})
	}
}

func TestMongoURIFromJSON(t *testing.T) {
	cfgJSON := []byte(`{
		"uri": "mongodb://127.0.0.1:27017/foo",
		"options": [
			{"compressors": "snappy,zlib"},
			{"replica_set": "v34_queue"},
			{"read_preference": "secondaryPreferred"},
			{"max_pool_size": 5}
		]
	}`)
	var conf mongoutil.Config
	// When
	err := json.Unmarshal(cfgJSON, &conf)
	// Then
	require.NoError(t, err)
	require.Equal(t,
		"mongodb://127.0.0.1:27017/foo?compressors=snappy,zlib&replicaSet=v34_queue&"+
			"readPreference=secondaryPreferred&maxPoolSize=5", conf.URIWithOptions())
}

func TestMongoURIFromYAML(t *testing.T) {
	cfgYAML := []byte(`servers:
    - mongo-routes-n01-us-east-1.postgun.com:27017
    - mongo-routes-n02-us-east-1.postgun.com:27017
    - mongo-routes-n03-us-east-1.postgun.com:27017
database: mg_prod
options:
    - ssl: true
    - tlsCertificateKeyFile: /etc/mailgun/ssl/mongo.pem
    - tlsCAFile: /etc/mailgun/ssl/mongo-ca.crt
    - replicaSet: routes
    - readPreferenceTags: "dc:use1"
    - readPreferenceTags: "dc:usw2"
`)
	var conf mongoutil.Config
	// When
	err := yaml.Unmarshal(cfgYAML, &conf)
	// Then
	require.NoError(t, err)
	require.Equal(t, "mongodb://mongo-routes-n01-us-east-1.postgun.com:27017,"+
		"mongo-routes-n02-us-east-1.postgun.com:27017,mongo-routes-n03-us-east-1.postgun.com:27017/mg_prod?"+
		"tlsCertificateKeyFile=/etc/mailgun/ssl/mongo.pem&tlsCAFile=/etc/mailgun/ssl/mongo-ca.crt&"+
		"replicaSet=routes&readPreferenceTags=dc:use1&readPreferenceTags=dc:usw2", conf.URIWithOptions())
}
