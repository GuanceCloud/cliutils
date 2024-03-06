package funcs

import (
	"io"
	"net/http"
)

func HttpRequest(method string, url string, headers map[string]string) (map[string]interface{}, error) {
	client := &http.Client{}

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	respData := map[string]interface{}{
		"status_code": resp.StatusCode,
		"body":        body,
	}

	return respData, nil
}
