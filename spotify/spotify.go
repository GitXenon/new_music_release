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

type ArtistResponses struct {
	Artists []ArtistResponse `json:"artists"`
}

type ArtistResponse struct {
	ExternalUrls ExternalURLs `json:"external_urls"`
	Followers    struct {
		Href  string `json:"href"`
		Total int    `json:"total"`
	} `json:"followers"`
	Genres     []string `json:"genres"`
	Href       string   `json:"href"`
	ID         string   `json:"id"`
	Images     []Image  `json:"images"`
	Name       string   `json:"name"`
	Popularity int      `json:"popularity"`
	Type       string   `json:"type"`
	URI        string   `json:"uri"`
}

type Image struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

type ExternalURLs struct {
	Spotify string `json:"spotify"`
}

type Tracks struct {
	Href     string  `json:"href"`
	Limit    int     `json:"limit"`
	Next     string  `json:"next"`
	Offset   int     `json:"offset"`
	Previous string  `json:"previous"`
	Total    int     `json:"total"`
	Items    []Track `json:"items"`
}

type Restrictions struct {
	Reason string `json:"reason"`
}

type Track struct {
	Artists          []struct{}   `json:"artists"`
	AvailableMarkets []string     `json:"available_markets"`
	DiscNumber       int          `json:"disc_number"`
	DurationMS       int          `json:"duration_ms"`
	Explicit         bool         `json:"explicit"`
	ExternalURLs     ExternalURLs `json:"external_urls"`
	Href             string       `json:"href"`
	ID               string       `json:"id"`
	IsPlayable       bool         `json:"is_playable"`
	LinkedFrom       struct {
		ExternalURLs ExternalURLs `json:"external_urls"`
		Href         string       `json:"href"`
		ID           string       `json:"id"`
		Type         string       `json:"type"`
		URI          string       `json:"uri"`
	} `json:"linked_from"`
	Restrictions Restrictions `json:"restrictions"`
	Name         string       `json:"name"`
	PreviewURL   string       `json:"preview_url"`
	TrackNumber  int          `json:"track_number"`
	Type         string       `json:"type"`
	URI          string       `json:"uri"`
	IsLocal      bool         `json:"is_local"`
}

type Copyright struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

type ExternalIDs struct {
	ISRC string `json:"isrc"`
	EAN  string `json:"ean"`
	UPC  string `json:"upc"`
}

type AlbumResponses struct {
	Albums []AlbumResponse `json:"albums"`
}

type AlbumResponse struct {
	AlbumType            string       `json:"album_type"`
	TotalTracks          int          `json:"total_tracks"`
	AvailableMarkets     []string     `json:"available_markets"`
	ExternalURLs         ExternalURLs `json:"external_urls"`
	Href                 string       `json:"href"`
	ID                   string       `json:"id"`
	Images               []Image      `json:"images"`
	Name                 string       `json:"name"`
	ReleaseDate          string       `json:"release_date"`
	ReleaseDatePrecision string       `json:"release_date_precision"`
	Restrictions         Restrictions `json:"restrictions"`
	Type                 string       `json:"type"`
	URI                  string       `json:"uri"`
	Artists              []struct{}   `json:"artists"`
	Tracks               Tracks       `json:"tracks"`
	Copyrights           []Copyright  `json:"copyrights"`
	ExternalIDs          ExternalIDs  `json:"external_ids"`
	Genres               []string     `json:"genres"`
	Label                string       `json:"label"`
	Popularity           int          `json:"popularity"`
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
		log.Error().Int("status_code", resp.StatusCode).Stringer("url", req.URL).Bytes("body", body).Msg("Authorization code Spotify")
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
		log.Error().Int("status_code", resp.StatusCode).Stringer("url", req.URL).Bytes("body", body).Msg("Refresh token Spotify")
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
	log.Info().Str("platform", "Spotify").Msgf("Searching %s from %s", album.AlbumName, album.ArtistName)

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
		log.Error().Int("status_code", resp.StatusCode).Stringer("url", req.URL).Bytes("body", body).Msg("GetSpotifyAlbum")
		return fmt.Errorf("spotify: expected %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Parse the JSON response
	var searchResponse SearchResponse
	err = json.Unmarshal(body, &searchResponse)
	if err != nil {
		log.Error().Int("status_code", resp.StatusCode).Str("url", apiURL).Msg("")
		return err
	}

	// Extract the ID from the response and add it to the album
	if searchResponse.Albums.Total == 0 {
		return errors.New("no artist match")
	}

	// TODO:	There was a bug where the wrong item was matched, we need to have a smarter thing to make sure the selection is good
	//			Ex.: we could check if title is the same, tracklist is the same, release date, etc.

	album.Spotify = searchResponse.Albums.Items[0]
	return nil
}

func GetArtists(album *album.Album) error {
	batchSize := 50
	artistsLength := len(album.Spotify.Artists)
	accessToken := viper.GetString("spotify.access_token")

	log.Info().Str("platform", "Spotify").Int("nb_artists", artistsLength).Msgf("Getting artists info for %s", album.AlbumName)

	for k := 0; k < artistsLength; k += batchSize {
		end := k + batchSize
		if end > artistsLength {
			end = artistsLength
		}

		subsetArtists := album.Spotify.Artists[k:end]

		var artistsIds []string

		for i := 0; i < len(subsetArtists); i++ {
			artistsIds = append(artistsIds, subsetArtists[i].Id)
		}

		baseURL := "https://api.spotify.com/v1/artists?ids=" + strings.Join(artistsIds, ",")

		log.Debug().Str("base_url", baseURL).Strs("artists_ids", artistsIds).Msg("spotify.GetArtists")

		req, err := http.NewRequest("GET", baseURL, nil)
		if err != nil {
			return err
		}

		// Set the necessary headers
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

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
			return GetArtists(album)
		}

		if resp.StatusCode == http.StatusUnauthorized {
			err = GetRefreshToken()
			if err != nil {
				return nil
			}
			return GetArtists(album)
		}

		if resp.StatusCode != http.StatusOK {
			log.Error().Str("platform", "Spotify").Int("status_code", resp.StatusCode).Stringer("url", req.URL).Bytes("body", body).Msg("error during GetArtists")
			return fmt.Errorf("spotify: expected %d, got %d", http.StatusOK, resp.StatusCode)
		}

		// Parse the JSON response
		var artistResponses ArtistResponses
		err = json.Unmarshal(body, &artistResponses)
		if err != nil {
			log.Error().Str("platform", "Spotify").Int("status_code", resp.StatusCode).Str("url", baseURL).Msg("")
			return err
		}

		for i := 0; i < len(artistResponses.Artists); i++ {
			for j := 0; j < len(subsetArtists); j++ {
				if artistResponses.Artists[i].ID == subsetArtists[j].Id {
					log.Debug().Str("platform", "Spotify").Str("artist_id", subsetArtists[j].Id).Msgf("Popularity for %s: %d", subsetArtists[j].Name, artistResponses.Artists[i].Popularity)
					subsetArtists[j].Popularity = artistResponses.Artists[i].Popularity
				}
			}
		}
	}
	return nil
}

func GetAlbums(albums *[]album.Album) error {
	batchSize := 20
	albumsLength := len(*albums)
	accessToken := viper.GetString("spotify.access_token")

	log.Info().Str("platform", "Spotify").Msgf("Getting albums info for %d albums", albumsLength)

	for k := 0; k < albumsLength; k += batchSize {
		end := k + batchSize
		if end > albumsLength {
			end = albumsLength
		}

		subsetAlbums := (*albums)[k:end]

		var albumsIds []string

		for i := 0; i < len(subsetAlbums); i++ {
			albumsIds = append(albumsIds, subsetAlbums[i].Spotify.Id)
		}

		baseURL := "https://api.spotify.com/v1/albums?ids=" + strings.Join(albumsIds, ",")

		log.Debug().Str("base_url", baseURL).Strs("albums_ids", albumsIds).Int("album_count", len(albumsIds)).Msg("spotify.GetAlbums")

		req, err := http.NewRequest("GET", baseURL, nil)
		if err != nil {
			return err
		}

		// Set the necessary headers
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

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
			return GetAlbums(albums)
		}

		if resp.StatusCode == http.StatusUnauthorized {
			err = GetRefreshToken()
			if err != nil {
				return nil
			}
			return GetAlbums(albums)
		}
		if resp.StatusCode != http.StatusOK {
			log.Error().Int("status_code", resp.StatusCode).Stringer("url", req.URL).Bytes("body", body).Msg("GetArtists")
			return fmt.Errorf("spotify: expected %d, got %d", http.StatusOK, resp.StatusCode)
		}

		// Parse the JSON response
		var albumsResponses AlbumResponses
		err = json.Unmarshal(body, &albumsResponses)
		if err != nil {
			log.Error().Int("status_code", resp.StatusCode).Str("url", baseURL).Msg("")
			return err
		}

		for i := 0; i < len(albumsResponses.Albums); i++ {
			for j := 0; j < len(subsetAlbums); j++ {
				if albumsResponses.Albums[i].ID == subsetAlbums[j].Spotify.Id {
					genres := strings.Join(albumsResponses.Albums[i].Genres, ",")
					log.Debug().Str("album_id", subsetAlbums[j].Spotify.Id).Msgf("Popularity for %s: %d", subsetAlbums[j].AlbumName, albumsResponses.Albums[i].Popularity)
					if genres != "" {
						subsetAlbums[j].Genre = genres
					}
					subsetAlbums[j].Spotify.Popularity = albumsResponses.Albums[i].Popularity
				}
			}
		}
	}
	return nil
}
