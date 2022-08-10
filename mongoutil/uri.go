package mongoutil

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/mailgun/holster/v4/clock"
	"github.com/mailgun/holster/v4/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/x/bsonx"
)

type Config struct {
	Servers  []string                 `json:"servers"`
	Database string                   `json:"database"`
	URI      string                   `json:"uri"`
	Options  []map[string]interface{} `json:"options"`
}

func MongoURI() string {
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		return "mongodb://127.0.0.1:27017/mg_test"
	}
	return mongoURI
}

func (c Config) URIWithOptions() string {
	URI := c.URI

	// Create an URI using the Servers list and Database if provided
	if len(c.Servers) != 0 && c.Database != "" {
		URI = fmt.Sprintf("mongodb://%s/%s", strings.Join(c.Servers, ","), c.Database)
	} else if len(c.Servers) != 0 {
		URI = fmt.Sprintf("mongodb://%s/", strings.Join(c.Servers, ","))
	}

	type opt struct {
		key   string
		value string
	}
	baseURI := URI
	var options []opt

	// Parse options from the URI.
	qmIdx := strings.Index(URI, "?")
	if qmIdx > 0 {
		baseURI = URI[:qmIdx]
		for _, pair := range strings.Split(URI[qmIdx+1:], "&") {
			eqIdx := strings.Index(pair, "=")
			if eqIdx > 0 {
				options = append(options, opt{key: pair[:eqIdx], value: pair[eqIdx+1:]})
			}
		}
	}

	// NOTE: The options are an ordered list because mongo cares
	// about the order of some options like replica tag order.

	// Override URI options with config options.
	for _, o := range c.Options {
		for optName, optVal := range o {
			switch optVal := optVal.(type) {
			case int:
				options = append(options, opt{key: toCamelCase(optName), value: strconv.Itoa(optVal)})
			case float64:
				options = append(options, opt{key: toCamelCase(optName), value: strconv.Itoa(int(optVal))})
			case string:
				options = append(options, opt{key: toCamelCase(optName), value: optVal})
			}
		}
	}

	// Construct a URI as recognized by mgo.Dial
	firstOpt := true
	var buf bytes.Buffer

	// If base URI was provided but no database specified
	if len(baseURI) != 0 {
		// if baseURI doesn't already end with a `/`
		if !strings.HasSuffix(baseURI, "/") {
			// Inspect the last character
			last := baseURI[len(baseURI)-1]
			// If the last character is an integer then we assume that we are looking at the port number,
			// thus no database was provided.
			if _, err := strconv.Atoi(string(last)); err == nil {
				// We must append a `/` to the end of the string
				baseURI += "/"
			}
		}
	}

	buf.WriteString(baseURI)

	for i := range options {
		o := options[i]
		if firstOpt {
			buf.WriteRune('?')
			firstOpt = false
		} else {
			buf.WriteRune('&')
		}
		buf.WriteString(o.key)
		buf.WriteRune('=')
		buf.WriteString(o.value)
	}
	return buf.String()
}

func toCamelCase(s string) string {
	var buf bytes.Buffer
	capitalize := false
	for _, ch := range s {
		if ch == '_' {
			capitalize = true
			continue
		}
		if capitalize {
			capitalize = false
			buf.WriteRune(unicode.ToUpper(ch))
			continue
		}
		buf.WriteRune(ch)
	}
	return buf.String()
}

// TimeTagDocument can be used to add a creation timestamp attribute `_tim` and
// to add or update an `updated_at` timestamp when it is called with reference
// to a single mongo document.
// Cf. https://github.com/mailgun/flagman/blob/92ffbc4/flagman/model/mixin.py#L51-L71
// Unlike in the above example's Python implementation, this func must be explicitly called
// immediately after document creation and every subsequent update to provide the same utility.
//
// id should be the value of the `_id` field assigned by mongodb to every document by default
// c must be the mongo document collection that contains the document identified by the id argument
// postInit should be true when this is being called immediately after document creation
func TimeTagDocument(ctx context.Context, id primitive.ObjectID, c *mongo.Collection, postInit bool) error {
	ctx, cancel := context.WithTimeout(ctx, 10*clock.Second)
	defer cancel()

	var update primitive.M
	now := bsonx.DateTime(clock.Now().UTC().UnixNano() / 1e6)
	filter := bson.M{"_id": id}

	if postInit {
		update = bson.M{"$set": bson.M{"_tim": now, "updated_at": now}}
	} else {
		update = bson.M{"$set": bson.M{"updated_at": now}}
	}
	if _, err := c.FindOneAndUpdate(ctx, filter, update, nil).DecodeBytes(); err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.Wrap(err, fmt.Sprintf("document %s from collection %s not found", id, c.Name()))
		}
		return err
	}
	return nil
}
