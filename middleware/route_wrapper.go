package middleware

import "github.com/gofiber/fiber/v2"

// WrappedRouter implements fiber.Router and automatically applies a middleware to every route registered on it
type WrappedRouter struct {
	fiber.Router
	Mw fiber.Handler
}

func (w *WrappedRouter) Get(path string, handlers ...fiber.Handler) fiber.Router {
	return w.Router.Get(path, append([]fiber.Handler{w.Mw}, handlers...)...)
}
func (w *WrappedRouter) Post(path string, handlers ...fiber.Handler) fiber.Router {
	return w.Router.Post(path, append([]fiber.Handler{w.Mw}, handlers...)...)
}
func (w *WrappedRouter) Put(path string, handlers ...fiber.Handler) fiber.Router {
	return w.Router.Put(path, append([]fiber.Handler{w.Mw}, handlers...)...)
}
func (w *WrappedRouter) Delete(path string, handlers ...fiber.Handler) fiber.Router {
	return w.Router.Delete(path, append([]fiber.Handler{w.Mw}, handlers...)...)
}
func (w *WrappedRouter) Patch(path string, handlers ...fiber.Handler) fiber.Router {
	return w.Router.Patch(path, append([]fiber.Handler{w.Mw}, handlers...)...)
}
func (w *WrappedRouter) Options(path string, handlers ...fiber.Handler) fiber.Router {
	return w.Router.Options(path, append([]fiber.Handler{w.Mw}, handlers...)...)
}
func (w *WrappedRouter) Head(path string, handlers ...fiber.Handler) fiber.Router {
	return w.Router.Head(path, append([]fiber.Handler{w.Mw}, handlers...)...)
}
func (w *WrappedRouter) All(path string, handlers ...fiber.Handler) fiber.Router {
	return w.Router.All(path, append([]fiber.Handler{w.Mw}, handlers...)...)
}

func WithMiddleware(r fiber.Router, mw fiber.Handler) fiber.Router {
	return &WrappedRouter{Router: r, Mw: mw}
}
