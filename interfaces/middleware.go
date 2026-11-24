package interfaces

// Middleware wraps a HandlerFunc, for example to log or measure each task.
type Middleware func(HandlerFunc) HandlerFunc
