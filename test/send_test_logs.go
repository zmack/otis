package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"time"

	logsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	logsv1pb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

const (
	collectorURL = "http://localhost:4318"
)

func main() {
	time.Sleep(1 * time.Second)

	if err := sendLogs(); err != nil {
		log.Fatalf("Failed to send logs: %v", err)
	}

	log.Println("Successfully sent test logs!")
}

func sendLogs() error {
	now := time.Now()
	timestamp := uint64(now.UnixNano())

	req := &logsv1.ExportLogsServiceRequest{
		ResourceLogs: []*logsv1pb.ResourceLogs{
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
				ScopeLogs: []*logsv1pb.ScopeLogs{
					{
						LogRecords: []*logsv1pb.LogRecord{
							{
								TimeUnixNano: timestamp,
								SeverityNumber: logsv1pb.SeverityNumber_SEVERITY_NUMBER_INFO,
								SeverityText:   "INFO",
								Body: &commonv1.AnyValue{
									Value: &commonv1.AnyValue_StringValue{
										StringValue: "This is a test log message",
									},
								},
								Attributes: []*commonv1.KeyValue{
									{
										Key: "log.source",
										Value: &commonv1.AnyValue{
											Value: &commonv1.AnyValue_StringValue{
												StringValue: "test-script",
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
		return fmt.Errorf("failed to marshal logs request: %w", err)
	}

	resp, err := http.Post(collectorURL+"/v1/logs", "application/x-protobuf", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to send logs request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	log.Println("Successfully sent logs data")
	return nil
}
