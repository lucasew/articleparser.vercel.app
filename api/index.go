package api

import (
	"net/http"

	"github.com/lucasew/readability-web/internal/handler"
)

// Handler is the Vercel Serverless Function entrypoint.
// It delegates to the internal handler package.
func Handler(w http.ResponseWriter, r *http.Request) {
	handler.Handler(w, r)
}
