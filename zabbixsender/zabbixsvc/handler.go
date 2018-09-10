package zabbixsvc

import "net/http"

// Handler is a collection of all the service handlers.
type Handler struct {
	JSONHandler *JSONHandler
}

// ServeHTTP delegates a request to the appropriate subhandler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.JSONHandler.ServeHTTP(w, r)
}
