package deezer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"newmusicrelease/album"
	"time"

	"github.com/rs/zerolog/log"
)

type AlbumSearchResponse struct {
	Data  []album.DeezerAlbum `json:"data"`
	Total int                 `json:"total"`
}

func SearchAlbum(album *album.Album) error {
	log.Info().Str("platform", "Deezer").Msgf("Searching %s from %s", album.AlbumName, album.ArtistName)

	baseURL := "https://api.deezer.com/search/album"
	query := url.Values{}
	query.Set("q", fmt.Sprintf("artist:'%s' album:'%s'", album.ArtistName, album.AlbumName))
	query.Set("limit", "1")
	apiURL := fmt.Sprintf("%s?%s", baseURL, query.Encode())

	// Create the HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return err
	}

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

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status_code", resp.StatusCode).Stringer("url", req.URL).Bytes("body", body).Msg("")
		return fmt.Errorf("deezer: expected %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Parse the JSON response
	var searchResponse AlbumSearchResponse
	err = json.Unmarshal(body, &searchResponse)
	if err != nil {
		log.Error().Int("status_code", resp.StatusCode).Str("url", apiURL).Msg("")
		return err
	}

	// Extract the ID from the response and add it to the album
	if searchResponse.Total == 0 {
		return errors.New("no album match")
	}

	// TODO:	Make sure the first album is a match, sometimes it returns only a Single.

	album.Deezer = searchResponse.Data[0]
	return nil
}
