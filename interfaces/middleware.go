package interfaces

type Middleware func(HandlerFunc) HandlerFunc
