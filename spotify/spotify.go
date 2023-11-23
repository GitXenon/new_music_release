package spotify

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"newmusicrelease/album"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type Authorization struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

type Artists struct {
	Href     string           `json:"href"`
	Limit    int              `json:"limit"`
	Next     string           `json:"next"`
	Offset   int              `json:"offset"`
	Previous string           `json:"previous"`
	Total    int              `json:"total"`
	Items    []ArtistResponse `json:"items"`
}

type ArtistResponse struct {
	ExternalUrls struct {
		Spotify string `json:"spotify"`
	} `json:"external_urls"`
	Followers struct {
		Href  string `json:"href"`
		Total int    `json:"total"`
	} `json:"followers"`
	Genres []string `json:"genres"`
	Href   string   `json:"href"`
	ID     string   `json:"id"`
	Images []struct {
		URL    string `json:"url"`
		Height int    `json:"height"`
		Width  int    `json:"width"`
	} `json:"images"`
	Name       string `json:"name"`
	Popularity int    `json:"popularity"`
	Type       string `json:"type"`
	URI        string `json:"uri"`
}

type SearchResponse struct {
	Tracks     interface{}         `json:"tracks"`
	Artists    Artists             `json:"artists"`
	Albums     album.SpotifyAlbums `json:"albums"`
	Playlists  interface{}         `json:"playlists"`
	Shows      interface{}         `json:"shows"`
	Episodes   interface{}         `json:"episodes"`
	Audiobooks interface{}         `json:"audiobooks"`
}

func GetAccessToken() error {
	// Authorization Code Flow https://developer.spotify.com/documentation/web-api/tutorials/code-flow
	clientID := viper.GetString("spotify.client_id")
	clientSecret := viper.GetString("spotify.client_secret")
	code := viper.GetString("spotify.token")

	// If access_token && refresh_token is set then we don't need to request a new access token
	if len(viper.GetString("spotify.access_token")) > 1 && len(viper.GetString("spotify.refresh_token")) > 1 {
		return GetRefreshToken()
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
		return fmt.Errorf("access token Spotify: expected %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var authorization Authorization
	err = json.Unmarshal(body, &authorization)
	if err != nil {
		return err
	}

	viper.Set("spotify.refresh_token", authorization.RefreshToken)
	viper.Set("spotify.access_token", authorization.AccessToken)

	err = viper.WriteConfig()
	if err != nil {
		return err
	}

	log.Info().Msg("Spotify refresh_token and access_token has been successfully updated!")

	return nil
}

func GetRefreshToken() error {
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

	var authorization Authorization
	err = json.Unmarshal(body, &authorization)
	if err != nil {
		return err
	}

	if len(authorization.RefreshToken) > 1 {
		viper.Set("spotify.refresh_token", authorization.RefreshToken)

	}
	if len(authorization.AccessToken) > 1 {
		viper.Set("spotify.access_token", authorization.AccessToken)
	}

	err = viper.WriteConfig()
	if err != nil {
		return err
	}

	log.Info().Msg("Spotify refresh_token and access_token has been successfully refreshed!")

	return nil
}

func SearchAlbum(album *album.Album) error {
	baseURL := "https://api.spotify.com/v1/search"
	query := url.Values{}
	query.Set("q", fmt.Sprintf("%s artist:%s", album.AlbumName, album.ArtistName))
	query.Set("type", "album")
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
		return SearchAlbum(album)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		err = GetRefreshToken()
		if err != nil {
			return nil
		}
		return SearchAlbum(album)
	}
	if resp.StatusCode != http.StatusOK {
		log.Error().Int("StatusCode", resp.StatusCode).Stringer("URL", req.URL).Bytes("body", body).Msg("GetSpotifyAlbum")
		return fmt.Errorf("spotify: expected %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Parse the JSON response
	var searchResponse SearchResponse
	err = json.Unmarshal(body, &searchResponse)
	if err != nil {
		log.Error().Int("StatusCode", resp.StatusCode).Str("URL", apiURL).Msg("")
		return err
	}

	log.Debug().Int("StatusCode", resp.StatusCode).Stringer("URL", req.URL).Interface("searchResponse", searchResponse).Msg("Response from Spotify API in GetSpotifyAlbum")

	// Extract the ID from the response and add it to the album
	if searchResponse.Albums.Total == 0 {
		return errors.New("no artist match")
	}

	album.Spotify = searchResponse.Albums.Items[0]
	return nil
}

func GetArtist(album *album.Album) error {
	baseURL := "https://api.spotify.com/v1/artists/" + album.Spotify.Artists[0].Id

	req, err := http.NewRequest("GET", baseURL, nil)
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
		return GetArtist(album)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		err = GetRefreshToken()
		if err != nil {
			return nil
		}
		return GetArtist(album)
	}
	if resp.StatusCode != http.StatusOK {
		log.Error().Int("StatusCode", resp.StatusCode).Stringer("URL", req.URL).Bytes("body", body).Msg("GetArtist")
		return fmt.Errorf("spotify: expected %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Parse the JSON response
	var artistResponse ArtistResponse
	err = json.Unmarshal(body, &artistResponse)
	if err != nil {
		log.Error().Int("StatusCode", resp.StatusCode).Str("URL", baseURL).Msg("")
		return err
	}

	log.Debug().Int("StatusCode", resp.StatusCode).Stringer("URL", req.URL).Interface("artistResponse", artistResponse).Msg("Response from Spotify API in GetArtist")

	album.Spotify.Artists[0].Popularity = artistResponse.Popularity

	return nil
}
