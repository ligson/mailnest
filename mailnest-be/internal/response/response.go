package response

import (
	"encoding/json"
	"net/http"
)

type Envelope struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	HTTPCode int    `json:"httpCode"`
	Data     any    `json:"data"`
}

func JSON(w http.ResponseWriter, status int, success bool, message string, data any) {
	if data == nil {
		data = map[string]any{}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{
		Success:  success,
		Message:  message,
		HTTPCode: status,
		Data:     data,
	})
}

func OK(w http.ResponseWriter, message string, data any) {
	JSON(w, http.StatusOK, true, message, data)
}

func Created(w http.ResponseWriter, message string, data any) {
	JSON(w, http.StatusCreated, true, message, data)
}

func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, false, message, nil)
}
