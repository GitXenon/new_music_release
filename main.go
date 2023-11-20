package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type Album struct {
	AlbumArt         string
	ArtistName       string
	AlbumName        string
	Genre            string
	TidalURL         string
	ArtistPopularity int
}

// Artist represents the structure of an artist.
type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Main bool   `json:"main"`
}

// Image represents the structure of an image.
type Image struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Resource represents the structure of the resource.
type Resource struct {
	ID          string   `json:"id"`
	BarcodeID   string   `json:"barcodeId"`
	Title       string   `json:"title"`
	Artists     []Artist `json:"artists"`
	Duration    int      `json:"duration"`
	ReleaseDate string   `json:"releaseDate"`
	ImageCover  []Image  `json:"imageCover"`
}

// TidalResponse represents the structure of the Tidal API response.
type TidalResponse struct {
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

type TidalAuthorization struct {
	ExpiresIn   int    `json:"expires_in"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type SpotifyAuthorization struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func getLatestFridayDate() string {
	currentDate := time.Now()
	daysUntilFriday := int((2 + currentDate.Weekday()) % 7)
	latestFriday := currentDate.AddDate(0, 0, -daysUntilFriday)
	return latestFriday.Format("20060102")
}

func genreScraper(genre string, albums *[]Album) error {
	latestFriday := getLatestFridayDate()

	newReleasesURL := fmt.Sprintf("https://everynoise.com/new_releases_by_genre.cgi?region=US&albumsonly=true&style=cards&date=%s&genre=%s", latestFriday, strings.ReplaceAll(genre, " ", "+"))
	genreSelector := fmt.Sprintf("#%s", strings.ReplaceAll(genre, " ", ""))

	c := colly.NewCollector()

	c.OnHTML(genreSelector, func(e *colly.HTMLElement) {
		nextSibling := e.DOM.Next()

		nextSibling.Children().Each(func(i int, s *goquery.Selection) {
			nextHTMLElement := &colly.HTMLElement{
				Text:     s.Text(),
				DOM:      s,
				Request:  e.Request,
				Response: e.Response,
			}
			albumArt := nextHTMLElement.ChildAttr("span.play img.albumart", "src")
			artistName := nextHTMLElement.ChildText("a > b")
			albumName := nextHTMLElement.ChildText("a > i")
			newAlbum := Album{
				AlbumArt:   albumArt,
				ArtistName: artistName,
				AlbumName:  albumName,
				Genre:      genre,
			}
			*albums = append(*albums, newAlbum)
		})
	})

	c.OnRequest(func(r *colly.Request) {
		log.Info().Stringer("URL", r.URL).Msg("Visiting")
	})

	c.Visit(newReleasesURL)

	return nil
}

func emailSender(albums *[]Album) error {
	// Sender and recipients' emails
	from := viper.GetString("email")
	to := "xavierbussiere+testing@gmail.com"

	password := viper.GetString("password")

	smtpHost := viper.GetString("smtp_host")
	smtpPort := viper.GetInt("smtp_port")

	// Message to be sent
	subject := "Subject: Test for New Music Release\n"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"

	tmpl, err := template.New("newsletter.tmpl").ParseFiles("newsletter.tmpl")
	if err != nil {
		return err
	}

	var body bytes.Buffer

	err = tmpl.Execute(&body, albums)

	if err != nil {
		return err
	}

	// Set up authentication information
	auth := smtp.PlainAuth("", from, password, smtpHost)

	// Connect to the SMTP server
	err = smtp.SendMail(fmt.Sprintf("%s:%d", smtpHost, smtpPort), auth, from, []string{to}, []byte(subject+mime+body.String()))
	if err != nil {
		return err
	}
	log.Info().Msgf("✨ Email sent successfully to %s ✨", to)
	return nil
}

func TidalAuth() (string, error) {
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

	var tidalAuthorization TidalAuthorization
	err = json.Unmarshal(body, &tidalAuthorization)
	if err != nil {
		return "", err
	}

	return tidalAuthorization.AccessToken, nil
}

func getTidalURL(album *Album, authKey string) error {
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
		return getTidalURL(album, authKey)
	}

	// Parse the JSON response
	var tidalResponse TidalResponse
	err = json.Unmarshal(body, &tidalResponse)
	if err != nil {
		log.Error().Int("StatusCode", resp.StatusCode).Str("URL", apiURL).Msg("")
		return err
	}

	// Extract the ID from the response and add it to the album
	if tidalResponse.Albums == nil || len(tidalResponse.Albums) == 0 {
		log.Debug().Int("StatusCode", resp.StatusCode).Stringer("URL", req.URL).Interface("tidalResponse", tidalResponse).Msg("Response from Tidal API in getTidalURL")
		return errors.New("no album match")
	}

	id := tidalResponse.Albums[0].ID
	album.TidalURL = fmt.Sprintf("https://listen.tidal.com/album/%s", id)
	return nil
}

func requestAccessTokenSpotify() error {
	clientID := viper.GetString("spotify.client_id")
	clientSecret := viper.GetString("spotify.client_secret")
	code := viper.GetString("spotify.token")

	// If access_token && refresh_token is set then we don't need to request a new access token
	if len(viper.GetString("spotify.access_token")) > 1 && len(viper.GetString("spotify.refresh_token")) > 1 {
		return nil
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

func refreshTokenSpotify() error {
	refreshToken := viper.GetString("spotify.refresh_token")

	// Build the request
	url := "https://accounts.spotify.com/api/token"
	payload := strings.NewReader("grant_type=refresh_token&refresh_token=" + refreshToken)

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

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

func getPopularityArtistSpotify(album *Album) error {
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
		return getPopularityArtistSpotify(album)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		err = refreshTokenSpotify()
		if err != nil {
			return nil
		}
		return getPopularityArtistSpotify(album)
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

func remove(a []Album, i int) []Album {
	a[i] = a[len(a)-1]
	return a[:len(a)-1]
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	viper.SetConfigFile("config.yaml")

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("fatal error config file")
	}

	// Tidal Authorization
	authKey, err := TidalAuth()
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	// Spotify Authorization
	err = requestAccessTokenSpotify()
	if err != nil {
		log.Fatal().Err(err).Msg("Error while making a request for an access token with Spotify")
	}

	var albums []Album

	genres := []string{"electro"}
	//genres := []string{"german indie", "art pop", "phonk", "wonky", "rap", "indietronica", "rock", "new wave", "electro", "art pop", "hip hop", "indie soul"}

	for _, genre := range genres {
		err = genreScraper(genre, &albums)
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}

	// Remove copies of album
	for i := range albums {
		for j := i + 1; j < len(albums); j++ {
			if albums[i].AlbumName == albums[j].AlbumName && albums[i].ArtistName == albums[j].ArtistName {
				albums[i].Genre = albums[i].Genre + ", " + albums[j].Genre
				albums = remove(albums, j)
			}
		}
	}

	for i := range albums {
		err = getTidalURL(&albums[i], authKey)
		if err != nil {
			log.Error().Err(err).Msg("error ecountered while getting the Tidal link")
		}
		err = getPopularityArtistSpotify(&albums[i])
		if err != nil {
			log.Error().Err(err).Msg("error ecountered while getting the Spotify's artist popularity")
		}
		log.Info().Interface("album", albums[i]).Msg("")
	}

	sort.Slice(albums, func(i, j int) bool {
		return albums[j].ArtistPopularity < albums[i].ArtistPopularity
	})

	err = emailSender(&albums)
	if err != nil {
		log.Error().Err(err).Msg("error ecountered during sending email")
	}
}
