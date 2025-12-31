package collector

import (
	"fmt"
	"io"
	"log"
	"net/http"

	logsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type LogsHandler struct {
	writer *FileWriter
}

func NewLogsHandler(writer *FileWriter) *LogsHandler {
	return &LogsHandler{
		writer: writer,
	}
}

func (h *LogsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	req := &logsv1.ExportLogsServiceRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		log.Printf("Failed to unmarshal logs request: %v", err)
		http.Error(w, "Failed to unmarshal request", http.StatusBadRequest)
		return
	}

	jsonData := protojson.MarshalOptions{
		Multiline:       false,
		Indent:          "",
		EmitUnpopulated: false,
	}.Format(req)

	if err := h.writer.WriteJSON(map[string]string{"data": jsonData}); err != nil {
		log.Printf("Failed to write logs data: %v", err)
		http.Error(w, "Failed to write data", http.StatusInternalServerError)
		return
	}

	resp := &logsv1.ExportLogsServiceResponse{}
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

	log.Printf("Received and stored logs data with %d resource logs", len(req.ResourceLogs))
}

func (h *LogsHandler) String() string {
	return fmt.Sprintf("LogsHandler{writer: %v}", h.writer)
}
