package proxy

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

	"github.com/aaronriekenberg/pi-web/config"
	"github.com/aaronriekenberg/pi-web/templates"
	"github.com/aaronriekenberg/pi-web/utils"
)

type proxyHTMLData struct {
	ProxyInfo *config.ProxyInfo
}

func proxyHTMLHandlerFunc(
	configuration *config.Configuration, proxyInfo config.ProxyInfo) http.HandlerFunc {

	cacheControlValue := configuration.TemplatePageInfo.CacheControlValue

	proxyHTMLData := &proxyHTMLData{
		ProxyInfo: &proxyInfo,
	}

	var builder strings.Builder
	if err := templates.Templates.ExecuteTemplate(&builder, templates.ProxyTemplateFile, proxyHTMLData); err != nil {
		log.Fatalf("Error executing proxy template ID %v: %v", proxyInfo.ID, err)
	}

	htmlString := builder.String()
	lastModified := time.Now()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(utils.CacheControlHeaderKey, cacheControlValue)
		w.Header().Add(utils.ContentTypeHeaderKey, utils.ContentTypeTextHTML)
		http.ServeContent(w, r, templates.ProxyTemplateFile, lastModified, strings.NewReader(htmlString))
	}
}

type proxyAPIResponse struct {
	ProxyInfo        *config.ProxyInfo `json:"proxyInfo"`
	Now              string            `json:"now"`
	ProxyDuration    string            `json:"proxyDuration"`
	ProxyStatus      string            `json:"proxyStatus"`
	ProxyRespHeaders http.Header       `json:"proxyRespHeaders"`
	ProxyOutput      string            `json:"proxyOutput"`
}

func makeProxyRequest(ctx context.Context, proxyInfo *config.ProxyInfo) (response *proxyAPIResponse, err error) {
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
		ProxyInfo:        proxyInfo,
		Now:              utils.FormatTime(proxyEndTime),
		ProxyDuration:    proxyDuration,
		ProxyStatus:      proxyResponse.Status,
		ProxyRespHeaders: proxyResponse.Header,
		ProxyOutput:      string(bodyBuffer),
	}
	return
}

func proxyAPIHandlerFunc(proxyInfo config.ProxyInfo) http.HandlerFunc {
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

		w.Header().Add(utils.ContentTypeHeaderKey, utils.ContentTypeApplicationJSON)
		w.Header().Add(utils.CacheControlHeaderKey, utils.MaxAgeZero)
		io.Copy(w, bytes.NewReader(jsonText))
	}
}

func CreateProxyHandler(configuration *config.Configuration, serveMux *http.ServeMux) {
	for _, proxyInfo := range configuration.Proxies {
		apiPath := "/api/proxies/" + proxyInfo.ID
		htmlPath := "/proxies/" + proxyInfo.ID + ".html"
		serveMux.Handle(
			htmlPath,
			proxyHTMLHandlerFunc(configuration, proxyInfo))
		serveMux.Handle(
			apiPath,
			proxyAPIHandlerFunc(proxyInfo))
	}
}
