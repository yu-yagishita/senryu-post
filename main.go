package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yu-yagishita/senryu-post/api"
	"github.com/yu-yagishita/senryu-post/db"
	"github.com/yu-yagishita/senryu-post/db/mongodb"

	"github.com/go-kit/kit/log"
	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	httptransport "github.com/go-kit/kit/transport/http"
)

func init() {
	db.Register("mongodb", &mongodb.Mongo{})
}

func main() {
	var (
		listen = flag.String("listen", ":8080", "HTTP listen address")
	)
	flag.Parse()

	var logger log.Logger
	logger = log.NewLogfmtLogger(os.Stderr)
	logger = log.With(logger, "listen", *listen, "caller", log.DefaultCaller)

	dbconn := false
	for !dbconn {
		err := db.Init()
		if err != nil {
			if err == db.ErrNoDatabaseSelected {
				logger.Log(err)
			}
			logger.Log(err)
			fmt.Println(err)
		} else {
			dbconn = true
		}
	}

	fieldKeys := []string{"method", "error"}
	requestCount := kitprometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Namespace: "my_group",
		Subsystem: "string_service",
		Name:      "request_count",
		Help:      "Number of requests received.",
	}, fieldKeys)
	requestLatency := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
		Namespace: "my_group",
		Subsystem: "string_service",
		Name:      "request_latency_microseconds",
		Help:      "Total duration of requests in microseconds.",
	}, fieldKeys)
	countResult := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
		Namespace: "my_group",
		Subsystem: "string_service",
		Name:      "count_result",
		Help:      "The result of each count method.",
	}, []string{})

	var svc api.Service
	svc = api.NewFixedService()
	svc = api.LoggingMiddleware(logger)(svc)
	svc = api.InstrumentingMiddleware(requestCount, requestLatency, countResult)(svc)

	getAllHandler := httptransport.NewServer(
		api.MakeGetAllEndpoint(svc),
		api.DecodeGetAllRequest,
		api.EncodeResponse,
	)

	http.Handle("/getAll", getAllHandler)
	http.Handle("/metrics", promhttp.Handler())
	logger.Log("msg", "HTTP", "addr", *listen)
	logger.Log("err", http.ListenAndServe(*listen, nil))
}
