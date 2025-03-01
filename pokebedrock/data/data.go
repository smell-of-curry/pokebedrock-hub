package data

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// UserNotFound ...
var UserNotFound = fmt.Errorf("user not found")

const url = "http://127.0.0.1:4000/"

// Roles ...
func Roles(xuid string) ([]string, error) {
	resp, err := http.Get(url + xuid)
	if err != nil || resp.StatusCode == http.StatusNotFound {
		return nil, UserNotFound
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var roles []string
	if err = json.Unmarshal(body, &roles); err != nil {
		return nil, err
	}
	return roles, nil
}
