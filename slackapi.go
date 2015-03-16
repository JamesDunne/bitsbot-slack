package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

func slackAPI(method string, params map[string]string) (map[string]interface{}, error) {
	// Format URL:
	u := fmt.Sprintf("https://slack.com/api/%s?token=%s", method, url.QueryEscape(env["SLACK_TOKEN"]))
	if params != nil {
		// Add extra params:
		for p, v := range params {
			u += fmt.Sprintf("&%s=%s", url.QueryEscape(p), url.QueryEscape(v))
		}
	}

	// Make the call:
	rsp, err := http.Get(u)
	if err != nil {
		return nil, err
	}

	// Read JSON response:
	wsResponse := make(map[string]interface{})
	err = json.NewDecoder(rsp.Body).Decode(&wsResponse)
	if err != nil {
		return nil, err
	}

	// Return error response:
	if !wsResponse["ok"].(bool) {
		return wsResponse, fmt.Errorf("%s", wsResponse["error"])
	}

	return wsResponse, nil
}
