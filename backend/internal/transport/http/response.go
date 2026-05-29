package httptransport

import (
	"encoding/json"
	"net/http"
)

func SendJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func SendErr(w http.ResponseWriter, status int, msg string) {
	SendJSON(w, status, map[string]string{"error": msg})
}
