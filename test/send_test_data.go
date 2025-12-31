package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"time"

	tracev1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	metricsv1 "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
	tracev1pb "go.opentelemetry.io/proto/otlp/trace/v1"
	metricsv1pb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/proto"
)

const (
	collectorURL = "http://localhost:4318"
)

func main() {
	time.Sleep(1 * time.Second)

	if err := sendTraces(); err != nil {
		log.Fatalf("Failed to send traces: %v", err)
	}

	if err := sendMetrics(); err != nil {
		log.Fatalf("Failed to send metrics: %v", err)
	}

	log.Println("Successfully sent test data!")
}

func sendTraces() error {
	now := time.Now()
	startTime := uint64(now.UnixNano())
	endTime := uint64(now.Add(100 * time.Millisecond).UnixNano())

	req := &tracev1.ExportTraceServiceRequest{
		ResourceSpans: []*tracev1pb.ResourceSpans{
			{
				Resource: &resourcev1.Resource{
					Attributes: []*commonv1.KeyValue{
						{
							Key: "service.name",
							Value: &commonv1.AnyValue{
								Value: &commonv1.AnyValue_StringValue{
									StringValue: "test-service",
								},
							},
						},
					},
				},
				ScopeSpans: []*tracev1pb.ScopeSpans{
					{
						Spans: []*tracev1pb.Span{
							{
								TraceId:           []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
								SpanId:            []byte{1, 2, 3, 4, 5, 6, 7, 8},
								Name:              "test-span",
								Kind:              tracev1pb.Span_SPAN_KIND_INTERNAL,
								StartTimeUnixNano: startTime,
								EndTimeUnixNano:   endTime,
								Attributes: []*commonv1.KeyValue{
									{
										Key: "test.attribute",
										Value: &commonv1.AnyValue{
											Value: &commonv1.AnyValue_StringValue{
												StringValue: "test-value",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal trace request: %w", err)
	}

	resp, err := http.Post(collectorURL+"/v1/traces", "application/x-protobuf", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to send trace request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	log.Println("Successfully sent trace data")
	return nil
}

func sendMetrics() error {
	now := time.Now()
	timestamp := uint64(now.UnixNano())

	req := &metricsv1.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricsv1pb.ResourceMetrics{
			{
				Resource: &resourcev1.Resource{
					Attributes: []*commonv1.KeyValue{
						{
							Key: "service.name",
							Value: &commonv1.AnyValue{
								Value: &commonv1.AnyValue_StringValue{
									StringValue: "test-service",
								},
							},
						},
					},
				},
				ScopeMetrics: []*metricsv1pb.ScopeMetrics{
					{
						Metrics: []*metricsv1pb.Metric{
							{
								Name:        "test.counter",
								Description: "A test counter metric",
								Data: &metricsv1pb.Metric_Sum{
									Sum: &metricsv1pb.Sum{
										AggregationTemporality: metricsv1pb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
										IsMonotonic:            true,
										DataPoints: []*metricsv1pb.NumberDataPoint{
											{
												TimeUnixNano: timestamp,
												Value: &metricsv1pb.NumberDataPoint_AsInt{
													AsInt: 42,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics request: %w", err)
	}

	resp, err := http.Post(collectorURL+"/v1/metrics", "application/x-protobuf", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to send metrics request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	log.Println("Successfully sent metrics data")
	return nil
}
