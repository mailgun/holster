### GRPCSrvBuilder
This build returns a GRPC resolver which will preform an DNS SRV record lookup on the provided domain name. Combined
with round-robin load balancing, it will instruct the GRPC client to load balance each request to every node returned
by the DNS SRV lookup.

```go

package main

import (
	"context"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/mailgun/holster/v4/discovery"
	"github.com/mailgun/ratelimits/v2"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
)

func main() {
	// Optional, will log the list of addresses dns-srv discovered when requesting the SRV records
	discovery.GRPCSrvLogAddresses = false

	// Optional, allows you to override the default logger. dns-srv will log an error when it is 
	// unable to request the SRV records.
	discovery.GRPCSrvDefaultLogger = logrus.New()
	
	// Required to register `dns-srv:///` as a schema GRPC can understand
	resolver.Register(discovery.NewGRPCSRVBuilder())

	c, err := grpc.Dial("dns-srv:///ratelimits-grpc.service.us-east4.prod.mailforce:8201",
		// Enable round-robin load balancing
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`),
		grpc.WithInsecure(),
	)
	if err != nil {
		panic(err)
	}

	rl := ratelimits.NewV1Client(c)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	resp, err := rl.HealthCheck(ctx, &ratelimits.HealthCheckReq{})
	if err != nil {
		panic(err)
	}
	spew.Dump(resp)

	// It can take some time for dns-srv to fetch all and resolve all the SRV
	// records for a service, especially if there are a lot of them.
	time.Sleep(time.Second * 15)

	// These requests should round-robin
	resp, err = rl.HealthCheck(ctx, &ratelimits.HealthCheckReq{})
	if err != nil {
		panic(err)
	}
	spew.Dump(resp)
	resp, _ = rl.HealthCheck(ctx, &ratelimits.HealthCheckReq{})
	spew.Dump(resp)
	resp, _ = rl.HealthCheck(ctx, &ratelimits.HealthCheckReq{})
	spew.Dump(resp)
	resp, _ = rl.HealthCheck(ctx, &ratelimits.HealthCheckReq{})
	spew.Dump(resp)
	
	c.Close()
}
```
