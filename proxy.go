package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

type proxyHTMLData struct {
	*proxyInfo
}

func proxyHTMLHandlerFunc(
	configuration *configuration, proxyInfo proxyInfo) http.HandlerFunc {

	cacheControlValue := configuration.TemplatePageInfo.CacheControlValue

	proxyHTMLData := &proxyHTMLData{
		proxyInfo: &proxyInfo,
	}

	var builder strings.Builder
	if err := templates.ExecuteTemplate(&builder, proxyTemplateFile, proxyHTMLData); err != nil {
		log.Fatalf("Error executing proxy template ID %v: %v", proxyInfo.ID, err)
	}

	htmlString := builder.String()
	lastModified := time.Now()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(cacheControlHeaderKey, cacheControlValue)
		w.Header().Add(contentTypeHeaderKey, contentTypeTextHTML)
		http.ServeContent(w, r, proxyTemplateFile, lastModified, strings.NewReader(htmlString))
	}
}

type proxyAPIResponse struct {
	*proxyInfo
	Now              string      `json:"now"`
	ProxyDuration    string      `json:"proxyDuration"`
	ProxyStatus      string      `json:"proxyStatus"`
	ProxyRespHeaders http.Header `json:"proxyRespHeaders"`
	ProxyOutput      string      `json:"proxyOutput"`
}

func makeProxyRequest(ctx context.Context, proxyInfo *proxyInfo) (response *proxyAPIResponse, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	defer cancel()

	httpRequest, err := http.NewRequestWithContext(ctx, "GET", proxyInfo.URL, nil)
	if err != nil {
		return
	}

	proxyStartTime := time.Now()
	proxyResponse, err := http.DefaultClient.Do(httpRequest)
	proxyEndTime := time.Now()

	if err != nil {
		return
	}

	defer proxyResponse.Body.Close()

	bodyBuffer, err := ioutil.ReadAll(proxyResponse.Body)
	if err != nil {
		return
	}

	proxyDuration := fmt.Sprintf("%.9f sec", proxyEndTime.Sub(proxyStartTime).Seconds())

	response = &proxyAPIResponse{
		proxyInfo:        proxyInfo,
		Now:              formatTime(proxyEndTime),
		ProxyDuration:    proxyDuration,
		ProxyStatus:      proxyResponse.Status,
		ProxyRespHeaders: proxyResponse.Header,
		ProxyOutput:      string(bodyBuffer),
	}
	return
}

func proxyAPIHandlerFunc(proxyInfo proxyInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proxyAPIResponse, err := makeProxyRequest(ctx, &proxyInfo)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		jsonText, err := json.Marshal(proxyAPIResponse)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Add(contentTypeHeaderKey, contentTypeApplicationJSON)
		w.Header().Add(cacheControlHeaderKey, maxAgeZero)
		io.Copy(w, bytes.NewReader(jsonText))
	}
}
