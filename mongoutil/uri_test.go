package mongoutil_test

import (
	"encoding/json"
	"testing"

	"github.com/mailgun/holster/v4/mongoutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		name: "URI parameters overridden with explicit ones",
		cfg: mongoutil.Config{
			URI: "mongodb://127.0.0.1:27017/foo?replicaSet=bazz&blah=wow",
			Options: map[string]interface{}{
				"replica_set":   "bar",
				"max_pool_size": 5,
			},
		},
		uri: "mongodb://127.0.0.1:27017/foo?blah=wow&maxPoolSize=5&replicaSet=bar",
	}, {
		name: "Read preference provided",
		cfg: mongoutil.Config{
			URI: "mongodb://127.0.0.1:27017/foo",
			Options: map[string]interface{}{
				"read_preference": "secondaryPreferred",
			},
		},
		uri: "mongodb://127.0.0.1:27017/foo?readPreference=secondaryPreferred",
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
		"options": {
			"compressors": "snappy,zlib",
			"replica_set": "v34_queue",
			"read_preference": "secondaryPreferred",
			"max_pool_size": 5
		}
	}`)
	var conf mongoutil.Config
	// When
	err := json.Unmarshal(cfgJSON, &conf)
	// Then
	require.NoError(t, err)
	require.Equal(t, "mongodb://127.0.0.1:27017/foo?compressors=snappy,zlib&maxPoolSize=5&readPreference=secondaryPreferred&replicaSet=v34_queue", conf.URIWithOptions())
}
