package mainpage

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/aaronriekenberg/pi-web/config"
	"github.com/aaronriekenberg/pi-web/environment"
	"github.com/aaronriekenberg/pi-web/templates"
	"github.com/aaronriekenberg/pi-web/utils"
)

type mainPageMetadata struct {
	Configuration                  *config.Configuration
	NumStaticDirectoriesInMainPage int
	Environment                    environment.Environment
	LastModified                   string
}

func buildMainPageString(configuration *config.Configuration, lastModified time.Time) string {
	var builder strings.Builder

	mainPageMetadata := &mainPageMetadata{
		Configuration: configuration,
		Environment:   environment.GetEnvironment(),
		LastModified:  utils.FormatTime(lastModified),
	}

	for i := range configuration.StaticDirectories {
		if configuration.StaticDirectories[i].IncludeInMainPage {
			mainPageMetadata.NumStaticDirectoriesInMainPage++
		}
	}

	if err := templates.Templates.ExecuteTemplate(&builder, templates.MainTemplateFile, mainPageMetadata); err != nil {
		log.Fatalf("error executing main page template %v", err)
	}
	return builder.String()
}

func mainPageHandlerFunc(configuration *config.Configuration) http.HandlerFunc {
	lastModified := time.Now()
	mainPageString := buildMainPageString(configuration, lastModified)
	cacheControlValue := configuration.TemplatePageInfo.CacheControlValue

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Add(utils.CacheControlHeaderKey, cacheControlValue)
		w.Header().Add(utils.ContentTypeHeaderKey, utils.ContentTypeTextHTML)
		http.ServeContent(w, r, templates.MainTemplateFile, lastModified, strings.NewReader(mainPageString))
	}
}

func CreateMainPageHandler(configuration *config.Configuration, serveMux *http.ServeMux) {
	serveMux.Handle("/", mainPageHandlerFunc(configuration))
}
