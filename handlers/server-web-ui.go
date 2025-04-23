package handlers

import (
	"embed"
	"net/http"
)

var content embed.FS

func StartServer(addr string) error {
	fs := http.FS(content)
	fileServer := http.FileServer(fs)

	http.Handle("/", fileServer)
	return http.ListenAndServe(addr, nil)
}
