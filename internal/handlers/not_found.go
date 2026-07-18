package handlers

import (
	"fmt"
	"net/http"
)

// HandleNotFound handles 404 errors with a custom XML response
func HandleNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.WriteHeader(http.StatusNotFound)

	// Clean path to remove leading slash for Key
	key := r.URL.Path
	if len(key) > 0 && key[0] == '/' {
		key = key[1:]
	}

	xmlResponse := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>The specified key does not exist.</Message>
  <RequestId>69870680B0CAA23639B92A8C</RequestId>
  <HostId>surrit.oss-eu-central-1.aliyuncs.com</HostId>
  <Key>%s</Key>
  <EC>0026-00000001</EC>
  <RecommendDoc>https://api.alibabacloud.com/troubleshoot?q=0026-00000001</RecommendDoc>
</Error>`, key)

	w.Write([]byte(xmlResponse))
}
