package tidal

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"newmusicrelease/aggregator"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Main bool   `json:"main"`
}

type ImageCover struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type Resource struct {
	ID          string       `json:"id"`
	BarcodeID   string       `json:"barcodeId"`
	Title       string       `json:"title"`
	Artists     []Artist     `json:"artists"`
	Duration    int          `json:"duration"`
	ReleaseDate string       `json:"releaseDate"`
	ImageCover  []ImageCover `json:"imageCover"`
}

type Response struct {
	Albums []struct {
		Resource Resource `json:"resource"`
		ID       string   `json:"id"`
		Status   int      `json:"status"`
		Message  string   `json:"message"`
	} `json:"albums"`
	Artists []interface{} `json:"artists"`
	Tracks  []interface{} `json:"tracks"`
	Videos  []interface{} `json:"videos"`
}

type Authorization struct {
	ExpiresIn   int    `json:"expires_in"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

func GetAuthorization() (string, error) {
	clientID := viper.GetString("tidal.client_id")
	clientSecret := viper.GetString("tidal.client_secret")

	b64Creds := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))

	// Build the request
	url := "https://auth.tidal.com/v1/oauth2/token"
	payload := strings.NewReader("grant_type=client_credentials")

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return "", err
	}

	// Set the Authorization header with the base64-encoded credentials
	req.Header.Set("Authorization", "Basic "+b64Creds)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Perform the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("StatusCode", resp.StatusCode).Stringer("URL", req.URL).Bytes("body", body).Msg("Authorization Tidal")
		return "", fmt.Errorf("auth Tidal: expected 200, got %d", resp.StatusCode)
	}

	var auth Authorization
	err = json.Unmarshal(body, &auth)
	if err != nil {
		return "", err
	}

	return auth.AccessToken, nil
}

func GetTidalURL(album *aggregator.Album, authKey string) error {
	// Build the Tidal API request URL
	baseURL := "https://openapi.tidal.com/search"
	query := url.Values{}
	query.Set("query", fmt.Sprintf("%s %s", album.AlbumName, album.ArtistName))
	query.Set("type", "ALBUMS")
	query.Set("offset", "0")
	query.Set("limit", "1")
	query.Set("countryCode", "US")
	query.Set("popularity", "WORLDWIDE")

	apiURL := fmt.Sprintf("%s?%s", baseURL, query.Encode())

	// Create the HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return err
	}

	// Set the necessary headers
	req.Header.Set("accept", "application/vnd.tidal.v1+json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authKey))
	req.Header.Set("Content-Type", "application/vnd.tidal.v1+json")

	// Perform the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		// TODO: Check if header contains more information about Retry-After
		sleepingTime, err := time.ParseDuration("3s")
		if err != nil {
			return err
		}
		time.Sleep(sleepingTime)
		return GetTidalURL(album, authKey)
	}

	// Parse the JSON response
	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		log.Error().Int("StatusCode", resp.StatusCode).Str("URL", apiURL).Msg("")
		return err
	}

	// Extract the ID from the response and add it to the album
	if response.Albums == nil || len(response.Albums) == 0 {
		log.Debug().Int("StatusCode", resp.StatusCode).Stringer("URL", req.URL).Interface("response", response).Msg("Response from Tidal API in getTidalURL")
		return errors.New("no album match")
	}

	id := response.Albums[0].ID
	album.TidalURL = fmt.Sprintf("https://listen.tidal.com/album/%s", id)
	return nil
}
