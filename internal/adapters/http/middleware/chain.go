package middleware

import "net/http"

// Middleware representa una función decoradora que envuelve un http.Handler.
type Middleware func(http.Handler) http.Handler

// Chain compone múltiples middlewares de forma declarativa y secuencial.
// La primera función en la lista envuelve a las siguientes, ejecutándose primero
// en el flujo entrante (inbound) y al final en el flujo saliente (outbound).
func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
