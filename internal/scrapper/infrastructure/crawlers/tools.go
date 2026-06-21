package crawlers

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	githubTemplate        = "https://api.github.com/repos"
	stackOverflowTemplate = "https://api.stackexchange.com/2.3/questions"

	githubUserAgent = "link-tracker"
)

func getJSON(client HTTPClient, req *http.Request, data any) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(data); err != nil {
		return err
	}

	return nil
}
