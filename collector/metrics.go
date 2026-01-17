package collector

import (
	"fmt"
	"io"
	"log"
	"net/http"

	metricsv1 "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type MetricsHandler struct {
	writer *FileWriter
}

func NewMetricsHandler(writer *FileWriter) *MetricsHandler {
	return &MetricsHandler{
		writer: writer,
	}
}

func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	req := &metricsv1.ExportMetricsServiceRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		log.Printf("Failed to unmarshal metrics request: %v", err)
		http.Error(w, "Failed to unmarshal request", http.StatusBadRequest)
		return
	}

	jsonData := protojson.MarshalOptions{
		Multiline:       false,
		Indent:          "",
		EmitUnpopulated: false,
	}.Format(req)

	if err := h.writer.WriteLine(jsonData); err != nil {
		log.Printf("Failed to write metrics data: %v", err)
		http.Error(w, "Failed to write data", http.StatusInternalServerError)
		return
	}

	resp := &metricsv1.ExportMetricsServiceResponse{}
	respData, err := proto.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	if _, err := w.Write(respData); err != nil {
		log.Printf("Failed to write response: %v", err)
	}

	log.Printf("Received and stored metrics data with %d resource metrics", len(req.ResourceMetrics))
}

func (h *MetricsHandler) String() string {
	return fmt.Sprintf("MetricsHandler{writer: %v}", h.writer)
}
