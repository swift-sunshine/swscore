package jaeger

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/models"
)

type JaegerServiceName = string

func buildJaegerServiceName(namespace, app string) JaegerServiceName {
	conf := config.Get()
	if conf.ExternalServices.Tracing.NamespaceSelector && namespace != conf.IstioNamespace {
		return app + "." + namespace
	}
	return app
}

func prepareQuery(u *url.URL, jsn JaegerServiceName, query models.TracingQuery) {
	q := url.Values{}
	q.Set("service", jsn)
	q.Set("start", query.StartMicros)
	if query.EndMicros != "" {
		q.Set("end", query.EndMicros)
	}
	if query.Tags != "" {
		q.Set("tags", query.Tags)
	}
	if query.MinDuration != "" {
		q.Set("minDuration", query.MinDuration)
	}
	if query.Limit > 0 {
		q.Set("limit", strconv.Itoa(query.Limit))
	}
	u.RawQuery = q.Encode()
}

func makeRequest(client http.Client, endpoint string, body io.Reader) (response []byte, status int, err error) {
	response = nil
	status = 0

	req, err := http.NewRequest(http.MethodGet, endpoint, body)
	if err != nil {
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	response, err = ioutil.ReadAll(resp.Body)
	status = resp.StatusCode
	return
}
