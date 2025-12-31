package collector

import (
	"fmt"
	"io"
	"log"
	"net/http"

	tracev1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type TraceHandler struct {
	writer *FileWriter
}

func NewTraceHandler(writer *FileWriter) *TraceHandler {
	return &TraceHandler{
		writer: writer,
	}
}

func (h *TraceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	req := &tracev1.ExportTraceServiceRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		log.Printf("Failed to unmarshal trace request: %v", err)
		http.Error(w, "Failed to unmarshal request", http.StatusBadRequest)
		return
	}

	jsonData := protojson.MarshalOptions{
		Multiline:       false,
		Indent:          "",
		EmitUnpopulated: false,
	}.Format(req)

	if err := h.writer.WriteJSON(map[string]string{"data": jsonData}); err != nil {
		log.Printf("Failed to write trace data: %v", err)
		http.Error(w, "Failed to write data", http.StatusInternalServerError)
		return
	}

	resp := &tracev1.ExportTraceServiceResponse{}
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

	log.Printf("Received and stored trace data with %d resource spans", len(req.ResourceSpans))
}

func (h *TraceHandler) String() string {
	return fmt.Sprintf("TraceHandler{writer: %v}", h.writer)
}
