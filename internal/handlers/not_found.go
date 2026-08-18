package handlers

import (
	"fmt"
	"net/http"
)

// HandleNotFound handles 404 errors with a custom XML response
func HandleNotFound(w http.ResponseWriter, r *http.Request) {
	HandleCachedError(w, r, http.StatusNotFound)
}

// HandleCachedError sends an error response that browsers do not retain while
// allowing Cloudflare to negative-cache it briefly to protect the origin.
func HandleCachedError(w http.ResponseWriter, r *http.Request, status int) {
	if status < http.StatusBadRequest {
		status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("CDN-Cache-Control", "public, max-age=60")
	w.WriteHeader(status)

	// Clean path to remove leading slash for Key
	key := r.URL.Path
	if len(key) > 0 && key[0] == '/' {
		key = key[1:]
	}

	code := "InternalError"
	message := http.StatusText(status)
	if status == http.StatusNotFound || status == http.StatusGone {
		code = "NoSuchKey"
		message = "The specified key does not exist."
	}

	xmlResponse := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>%s</Code>
  <Message>%s</Message>
  <RequestId>69870680B0CAA23639B92A8C</RequestId>
  <HostId>surrit.oss-eu-central-1.aliyuncs.com</HostId>
  <Key>%s</Key>
  <EC>0026-00000001</EC>
  <RecommendDoc>https://api.alibabacloud.com/troubleshoot?q=0026-00000001</RecommendDoc>
</Error>`, code, message, key)

	w.Write([]byte(xmlResponse))
}
