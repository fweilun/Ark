// README: Recovery middleware (stub for MVP).
package middleware

import "net/http"

func Recovery(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if recover() != nil {
                w.WriteHeader(http.StatusInternalServerError)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
