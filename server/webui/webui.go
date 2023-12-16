package webui

import (
	"net/http"
)

type Controller struct {
	*http.ServeMux

	webuipath        string
	resolveProxyPath ProxyPathResolver
}

type ProxyPathResolver func(in string) string

func New(webui string, resolveProxyPath ProxyPathResolver) (*Controller, error) {
	c := Controller{
		ServeMux:         http.NewServeMux(),
		webuipath:        webui,
		resolveProxyPath: resolveProxyPath,
	}

	c.Handle("/", http.FileServer(http.Dir(c.webuipath)))

	return &c, nil
}
