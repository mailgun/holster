package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

type QueryResponse struct {
	Status string       `json:"status"`
	Data   ResponseData `json:"data"`
}

type ResponseData struct {
	ResultType string   `json:"result_type"`
	Result     []Result `json:"result"`
}

type Result struct {
	Metric map[string]string `json:"metric"`
	Values [][]interface{}   `json:"values"`
}

type TimeSeries []DataPoint

type DataPoint struct {
	TimeStamp int64
	Value     string
}

type Query struct {
	Query string
	Start time.Time
	End   time.Time
	Step  string
}

func main() {
	os.Exit(runCmd())
}

func runCmd() int {
	var start, end, endpoint, cos, pod, payload string
	var err error

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	flag.StringVar(&endpoint, "endpoint", "",
		"The prometheus server to query")
	flag.StringVar(&start, "start", "", "Start time in the format '2021-08-16:00:00Z'")
	flag.StringVar(&end, "end", "", "Start time in the format '2021-09-01T00:00:00Z'")
	flag.StringVar(&cos, "cos", "bulk", "Specify the cos. Typical options are ['bulk', 'bulk-big', 'trans', 'trans-big'")
	flag.StringVar(&pod, "pod", "default", "Specify the pod. Typical options are ['default', 'rapidfire']")
	flag.StringVar(&payload, "payload", "smtp", "Specify the payload. Typical options are ['smtp', 'http']")
	flag.Parse()

	if start == "" {
		fmt.Printf("`-start` is required (See -h for usage)\n")
		return 1
	}

	if end == "" {
		fmt.Printf("`-end` is required (See -h for usage)\n")
		return 1
	}

	q := Query{
		Query: fmt.Sprintf(`max(querator_ttp_seconds{region=~"us-.*", pod=%q, payload=%q, cos=%q, quantile="0.95"})`, pod, payload, cos),
		Step:  "30m",
	}

	q.Start, err = time.Parse(time.RFC3339, start)
	if err != nil {
		panic(err)
	}

	q.End, err = time.Parse(time.RFC3339, end)
	if err != nil {
		panic(err)
	}

	ts, err := RunQuery(ctx, endpoint, q)
	if err != nil {
		panic(err)
	}

	var total float64
	for _, dp := range *ts {
		f, err := strconv.ParseFloat(dp.Value, 64)
		if err != nil {
			panic(fmt.Errorf("while strconv '%s': %w", dp.Value, err))
		}
		total += f
	}

	fmt.Printf("Total: %f Divided by: %d = Avg: %f seconds\n", total, len(*ts), total/float64(len(*ts)))
	return 0
}

func RunQuery(ctx context.Context, endpoint string, q Query) (*TimeSeries, error) {
	params := url.Values{}
	params.Add("query", q.Query)
	params.Add("start", fmt.Sprintf("%d", q.Start.Unix()))
	params.Add("end", fmt.Sprintf("%d", q.End.Unix()))
	params.Add("step", q.Step)

	u, err := url.Parse(fmt.Sprintf("%s/api/v1/query_range?%s", endpoint, params.Encode()))
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("while preparing request: %w", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("during http request: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("got non 200 response code %s: %s", res.Status, readAll(res.Body))
	}

	var qResp QueryResponse
	dec := json.NewDecoder(res.Body)
	if err := dec.Decode(&qResp); err != nil {
		return nil, fmt.Errorf("while decoding json response: %w", err)
	}

	var ts TimeSeries
	for _, r := range qResp.Data.Result {
		for _, v := range r.Values {
			ts = append(ts, DataPoint{
				TimeStamp: int64(v[0].(float64)),
				Value:     v[1].(string),
			})
		}
	}
	return &ts, nil
}

func readAll(r io.ReadCloser) string {
	b, _ := io.ReadAll(r)
	return string(b)
}
