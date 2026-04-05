package api

import "net/http"

// APIVersion is the current API version. Follows date-based versioning
// like Stripe (YYYY-MM-DD). Clients can pin to a version by sending
// the X-API-Version header.
const APIVersion = "2026-04-05"

// versionMiddleware sets the X-API-Version response header on every response
// and records the client-requested version for future compatibility handling.
func (s *Server) versionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-API-Version", APIVersion)
		next.ServeHTTP(w, r)
	})
}
