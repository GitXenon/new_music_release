package spotify

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

type SpotifyAuthorization struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

type SpotifyArtist struct {
	Uri        string `json:"uri"`
	Type       string `json:"type"`
	Popularity int    `json:"popularity"`
	Name       string `json:"name"`
	Id         string `json:"id"`
}

type SpotifyArtists struct {
	Href     string          `json:"href"`
	Limit    int             `json:"limit"`
	Next     string          `json:"next"`
	Offset   int             `json:"offset"`
	Previous string          `json:"previous"`
	Total    int             `json:"total"`
	Items    []SpotifyArtist `json:"items"`
}

type SpotifyResponse struct {
	Tracks     interface{}    `json:"tracks"`
	Artists    SpotifyArtists `json:"artists"`
	Albums     interface{}    `json:"albums"`
	Playlists  interface{}    `json:"playlists"`
	Shows      interface{}    `json:"shows"`
	Episodes   interface{}    `json:"episodes"`
	Audiobooks interface{}    `json:"audiobooks"`
}

func RequestAccessTokenSpotify() error {
	// Authorization Code Flow https://developer.spotify.com/documentation/web-api/tutorials/code-flow
	clientID := viper.GetString("spotify.client_id")
	clientSecret := viper.GetString("spotify.client_secret")
	code := viper.GetString("spotify.token")

	// If access_token && refresh_token is set then we don't need to request a new access token
	if len(viper.GetString("spotify.access_token")) > 1 && len(viper.GetString("spotify.refresh_token")) > 1 {
		return RefreshTokenSpotify()
	}

	// Build the request
	url := "https://accounts.spotify.com/api/token"
	payload := strings.NewReader("redirect_uri=https://open.spotify.com&grant_type=authorization_code&code=" + code)

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return err
	}

	b64Creds := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	req.Header.Set("Authorization", "Basic "+b64Creds)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("StatusCode", resp.StatusCode).Stringer("URL", req.URL).Bytes("body", body).Msg("Authorization code Spotify")
		return fmt.Errorf("access token Spotify: expected 200, got %d", resp.StatusCode)
	}

	var spotifyAuthorization SpotifyAuthorization
	err = json.Unmarshal(body, &spotifyAuthorization)
	if err != nil {
		return err
	}

	viper.Set("spotify.refresh_token", spotifyAuthorization.RefreshToken)
	viper.Set("spotify.access_token", spotifyAuthorization.AccessToken)

	err = viper.WriteConfig()
	if err != nil {
		return err
	}

	log.Info().Msg("Spotify refresh_token and access_token has been successfully updated!")

	return nil
}

func RefreshTokenSpotify() error {
	clientID := viper.GetString("spotify.client_id")
	clientSecret := viper.GetString("spotify.client_secret")
	refreshToken := viper.GetString("spotify.refresh_token")

	// Build the request
	url := "https://accounts.spotify.com/api/token"
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

	b64Creds := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	req.Header.Set("Authorization", "Basic "+b64Creds)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	q := req.URL.Query()
	q.Add("grant_type", "refresh_token")
	q.Add("refresh_token", refreshToken)
	req.URL.RawQuery = q.Encode()

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

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("StatusCode", resp.StatusCode).Stringer("URL", req.URL).Bytes("body", body).Msg("Refresh token Spotify")
		return fmt.Errorf("refresh token spotify: expected 200, got %d", resp.StatusCode)
	}

	var spotifyAuthorization SpotifyAuthorization
	err = json.Unmarshal(body, &spotifyAuthorization)
	if err != nil {
		return err
	}

	viper.Set("spotify.refresh_token", spotifyAuthorization.RefreshToken)
	viper.Set("spotify.access_token", spotifyAuthorization.AccessToken)

	err = viper.WriteConfig()
	if err != nil {
		return err
	}

	log.Info().Msg("Spotify refresh_token and access_token has been successfully refreshed!")

	return nil
}

func GetPopularityArtistSpotify(album *aggregator.Album) error {
	// Build the Tidal API request URL
	baseURL := "https://api.spotify.com/v1/search"
	query := url.Values{}
	query.Set("q", album.ArtistName)
	query.Set("type", "artist")
	query.Set("limit", "1")

	apiURL := fmt.Sprintf("%s?%s", baseURL, query.Encode())

	// Create the HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return err
	}

	// Set the necessary headers
	accessToken := viper.GetString("spotify.access_token")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer "+accessToken))

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
		sleepingTime, err := time.ParseDuration("3s")
		if err != nil {
			return err
		}
		time.Sleep(sleepingTime)
		return GetPopularityArtistSpotify(album)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		err = RefreshTokenSpotify()
		if err != nil {
			return nil
		}
		return GetPopularityArtistSpotify(album)
	}
	if resp.StatusCode != http.StatusOK {
		log.Error().Int("StatusCode", resp.StatusCode).Stringer("URL", req.URL).Bytes("body", body).Msg("getPopularityArtistSpotify")
		return fmt.Errorf("spotify: expected 200, got %d", resp.StatusCode)
	}

	// Parse the JSON response
	var spotifyResponse SpotifyResponse
	err = json.Unmarshal(body, &spotifyResponse)
	if err != nil {
		log.Error().Int("StatusCode", resp.StatusCode).Str("URL", apiURL).Msg("")
		return err
	}

	log.Debug().Int("StatusCode", resp.StatusCode).Stringer("URL", req.URL).Interface("spotifyResponse", spotifyResponse).Msg("Response from Spotify API in setPopularityArtistSpotify")

	// Extract the ID from the response and add it to the album
	if spotifyResponse.Artists.Total == 0 {
		return errors.New("no artist match")
	}

	popularity := spotifyResponse.Artists.Items[0].Popularity
	album.ArtistPopularity = popularity
	return nil
}
