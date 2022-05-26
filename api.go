package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

func powerStatus(ctx context.Context) (ps PowerStatus, err error) {
	var status Status
	// Construct URL from options
	u := fmt.Sprintf("%v/hubs/%v/status", cfg.HarmonyApi.Url, cfg.HarmonyApi.DefaultHub)
	data, err := request(ctx, http.MethodGet, u)
	if err != nil {
		return ps, err
	}
	err = json.Unmarshal(data, &status)
	if err != nil {
		return ps, err
	}
	ps.Off = status.Off
	ps.ExpectedActivity = status.CurrentActivity.Slug == cfg.HarmonyApi.DefaultActivity
	ps.Activity = status.CurrentActivity.Slug

	return ps, err
}

func turnOn(ctx context.Context) (ps PowerStatus, err error) {
	u := fmt.Sprintf("%v/hubs/%v/activities/%v", cfg.HarmonyApi.Url, cfg.HarmonyApi.DefaultHub, cfg.HarmonyApi.DefaultActivity)
	data, err := request(ctx, http.MethodPost, u)
	if err != nil {
		return ps, err
	}
	var status RequestResponse
	err = json.Unmarshal(data, &status)
	if err != nil {
		return ps, err
	}
	return powerStatus(ctx)
}

func turnOff(ctx context.Context) (ps PowerStatus, err error) {
	u := fmt.Sprintf("%v/hubs/%v/off", cfg.HarmonyApi.Url, cfg.HarmonyApi.DefaultHub)
	data, err := request(ctx, http.MethodPut, u)
	if err != nil {
		return ps, err
	}
	var status RequestResponse
	err = json.Unmarshal(data, &status)
	if err != nil {
		return ps, err
	}
	return powerStatus(ctx)

}

func request(ctx context.Context, method string, u string) (response []byte, err error) {
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return response, err
	}
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return response, err
	}
	if resp.StatusCode != http.StatusOK {
		return response, errors.New(fmt.Sprintf("upstream status code %d (request URI: %v)", resp.StatusCode, u))
	}
	response, err = ioutil.ReadAll(resp.Body)
	return response, nil
}

type PowerStatus struct {
	Off              bool   `json:"off"`
	Activity         string `json:"activity,omitempty"`
	ExpectedActivity bool   `json:"expected_activity,omitempty"`
}

type RequestResponse struct {
	Message string
}

type Status struct {
	Off              bool               `json:"off"`
	CurrentActivity  CurrentActivity    `json:"current_activity"`
	ActivityCommands []ActivitiyCommand `json:"activity_commands,omitempty"`
}
type CurrentActivity struct {
	ID           string `json:"id,omitempty"`
	Slug         string `json:"slug,omitempty"`
	Label        string `json:"label,omitempty"`
	IsAVActivity bool   `json:"isAVActivity,omitempty"`
}
type ActivitiyCommand struct {
	Name  string `json:"name,omitempty"`
	Slug  string `json:"slug,omitempty"`
	Label string `json:"label,omitempty"`
}
