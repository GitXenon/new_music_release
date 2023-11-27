package album

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
)

type SpotifyArtist struct {
	ExternalUrls struct {
		Spotify string `json:"spotify"`
	} `json:"external_urls"`
	Href       string `json:"href"`
	Id         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Uri        string `json:"uri"`
	Popularity int    `json:"popularity"`
}

type SpotifyAlbum struct {
	AlbumType        string   `json:"album_type"`
	TotalTracks      int      `json:"total_tracks"`
	AvailableMarkets []string `json:"available_markets"`
	ExternalUrls     struct {
		Spotify string `json:"spotify"`
	} `json:"external_urls"`
	Href                 string       `json:"href"`
	Id                   string       `json:"id"`
	Images               []ImageCover `json:"images"`
	Name                 string       `json:"name"`
	ReleaseDate          string       `json:"release_date"`
	ReleaseDatePrecision string       `json:"release_date_precision"`
	Restrictions         struct {
		Reason string `json:"reason"`
	} `json:"restrictions"`
	Type       string          `json:"type"`
	Genres     []string        `json:"genres"`
	Uri        string          `json:"uri"`
	Artists    []SpotifyArtist `json:"artists"`
	Popularity int
}

type SpotifyAlbums struct {
	Href     string         `json:"href"`
	Limit    int            `json:"limit"`
	Next     string         `json:"next"`
	Offset   int            `json:"offset"`
	Previous string         `json:"previous"`
	Total    int            `json:"total"`
	Items    []SpotifyAlbum `json:"items"`
}

type TidalArtist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Main bool   `json:"main"`
}

type ImageCover struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type TidalAlbum struct {
	ID              string        `json:"id"`
	BarcodeID       string        `json:"barcodeId"`
	Title           string        `json:"title"`
	Artists         []TidalArtist `json:"artists"`
	Duration        int           `json:"duration"`
	ReleaseDate     string        `json:"releaseDate"`
	ImageCover      []ImageCover  `json:"imageCover"`
	VideoCover      []ImageCover  `json:"videoCover"`
	NumberOfVolumes int           `json:"numberOfVolumes"`
	NumberOfTracks  int           `json:"numberOfTracks"`
	NumberOfVideos  int           `json:"numberOfVideos"`
	Type            string        `json:"type"`
	Copyright       string        `json:"copyright"`
	MediaMetadata   interface{}   `json:"mediaMetadata"`
	Properties      interface{}   `json:"properties"`
}

type Album struct {
	AlbumArt   string
	ArtistName string
	AlbumName  string
	Genre      string
	Tidal      TidalAlbum
	Spotify    SpotifyAlbum
}

func remove(a []Album, i int) []Album {
	a[i] = a[len(a)-1]
	return a[:len(a)-1]
}

func RemoveCopies(albums []Album) []Album {
	for i := 0; i < len(albums); i++ {
		if albums[i].AlbumName == "" {
			log.Debug().Str("album_name", albums[i].AlbumName).Str("artist_name", albums[i].ArtistName).Msg("Removed")
			albums = remove(albums, i)
		}
	}
	for i := range albums {
		for j := i + 1; j < len(albums); j++ {
			if albums[i].AlbumName == albums[j].AlbumName && albums[i].ArtistName == albums[j].ArtistName {
				log.Debug().Str("album_name", albums[i].AlbumName).Str("artist_name", albums[i].ArtistName).Strs("Genres", []string{albums[j].Genre, albums[i].Genre}).Msg("Found a duplicate")
				albums[i].Genre = strings.Join([]string{albums[j].Genre, albums[i].Genre}, ",")
				albums = remove(albums, j)
			}
		}
	}
	return albums
}

func RankByPopularity(albums []Album) []Album {
	sort.Slice(albums, func(i, j int) bool {
		return albums[j].Spotify.Popularity < albums[i].Spotify.Popularity
	})
	return albums
}

func (album Album) GetTidalURL() (string, error) {
	if album.Tidal.ID == "" {
		return "", nil
	}
	return fmt.Sprintf("https://listen.tidal.com/album/%s", album.Tidal.ID), nil
}

func (album Album) GetSpotifyURL() (string, error) {
	if album.Spotify.ExternalUrls.Spotify == "" {
		return "", nil
	}
	return album.Spotify.ExternalUrls.Spotify, nil
}
