package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	EnvVarNameNextService = "CHAINER_NEXT_SERVICE"
	EnvVarNameLoadPerReq  = "CHAINER_LOAD_PER_REQUEST"
	loadUnit              = 5000
)

var (
	nextSvc       = ""
	loadPerReq    = mustParseDuration("1ms")
	clientTimeout = mustParseDuration("500ms")
)

func mustParseDuration(t string) time.Duration {
	d, err := time.ParseDuration(t)
	if err != nil {
		panic(err)
	}
	return d
}

func wait() int {
	counter := 0
	cancel := make(chan any, 1)
	defer close(cancel)

	go func() {
		time.Sleep(loadPerReq)
		cancel <- ""
	}()

	for {
		select {
		case <-cancel:
			return counter
		default:
			for i := 0; i < loadUnit; i++ {
				counter++
			}
		}
	}
}

func main() {
	if val, ok := os.LookupEnv(EnvVarNameNextService); ok {
		nextSvc = val
	}
	if val, ok := os.LookupEnv(EnvVarNameLoadPerReq); ok {
		loadPerReq = mustParseDuration(val)
	}

	reg := prometheus.NewRegistry()

	responseSuccessTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "http_response_success_total",
		Help: "Total number of successful HTTP handler runs.",
	})
	responseErrorTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "http_response_error_total",
		Help: "Total number of HTTP handler errors.",
	})
	timeoutErrorTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "http_timeout_error_total",
		Help: "Total number of timeouts for downstream HTTP queries.",
	})

	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		responseSuccessTotal,
		responseErrorTotal,
		timeoutErrorTotal,
	)

	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	client := http.Client{Timeout: clientTimeout}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		counter := 0

		// run local workload
		counter += wait()

		// query next service
		if nextSvc != "" {
			nextResp, err := client.Get("http://" + nextSvc)
			if err != nil {
				if os.IsTimeout(err) {
					timeoutErrorTotal.Add(1)
					log.Printf("Timeout querying next-service %q: %s", nextSvc, err)
					http.Error(w, err.Error(), http.StatusRequestTimeout)
				} else {
					responseErrorTotal.Add(1)
					log.Printf("Error querying next-service %q: %s", nextSvc, err)
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				return
			}
			resp := make(map[string]int)
			if err := json.NewDecoder(nextResp.Body).Decode(&resp); err != nil {
				responseErrorTotal.Add(1)
				log.Printf("Error decoding JSON response from next-service %q: %s", nextSvc, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if c, ok := resp["counter"]; ok {
				counter += c
			}
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		resp := make(map[string]int)
		resp["counter"] = counter
		jsonResp, err := json.Marshal(resp)
		if err != nil {
			responseErrorTotal.Add(1)
			log.Printf("Error encoding JSON response: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(jsonResp)
		responseSuccessTotal.Add(1)
		return
	})

	log.Fatal(http.ListenAndServe(":8888", nil))

}
