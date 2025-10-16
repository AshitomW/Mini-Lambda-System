package main

import "github.com/prometheus/client_golang/prometheus"

var (
	metricsInvocations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:"Total_Invocations",
			Help: "Number of function invocations",
		},
		[]string{"function"},
	)

	metricsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:"Invocation Duration ms",
			Help: "Invocation latency in ms",
			Buckets: prometheus.LinearBuckets(10,100,10),
		},
		[]string{"function"},
	)
)

func init(){
	prometheus.MustRegister(metricsInvocations)
	prometheus.MustRegister(metricsDuration)
}