package main

import (
	"bytes"
	"fmt"
	"net/smtp"
	"newmusicrelease/album"
	"newmusicrelease/spotify"
	"newmusicrelease/tidal"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/jordan-wright/email"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func getLatestFridayDate() string {
	currentDate := time.Now()
	daysUntilFriday := int((2 + currentDate.Weekday()) % 7)
	latestFriday := currentDate.AddDate(0, 0, -daysUntilFriday)
	return latestFriday.Format("20060102")
}

func genreScraper(genre string, albums *[]album.Album) error {
	latestFriday := getLatestFridayDate()

	newReleasesURL := fmt.Sprintf("https://everynoise.com/new_releases_by_genre.cgi?region=US&albumsonly=true&style=cards&date=%s&genre=%s", latestFriday, strings.ReplaceAll(genre, " ", "+"))
	genreSelector := fmt.Sprintf("div#%s", strings.ReplaceAll(genre, " ", ""))

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
			newAlbum := album.Album{
				AlbumArt:   albumArt,
				ArtistName: artistName,
				AlbumName:  albumName,
				Genre:      genre,
			}
			if albumName != "" {
				log.Debug().Msgf("Added a new album: %s", albumName)
				*albums = append(*albums, newAlbum)
			}
		})
	})

	c.OnRequest(func(r *colly.Request) {
		log.Info().Stringer("url", r.URL).Msg("Visiting")
	})

	c.Visit(newReleasesURL)

	return nil
}

func emailSender(albums *[]album.Album) error {
	password := viper.GetString("password")
	smtpHost := viper.GetString("smtp_host")
	smtpPort := viper.GetInt("smtp_port")

	tmpl, err := template.New("newsletter.tmpl").ParseFiles("newsletter.tmpl")
	if err != nil {
		return err
	}

	var body bytes.Buffer

	err = tmpl.Execute(&body, albums)

	if err != nil {
		return err
	}

	f, err := os.Create("./template.html")
	if err != nil {
		return err
	}

	tmpl.Execute(f, albums)
	if err != nil {
		return err
	}

	f.Close()

	from := viper.GetString("email")
	to := "xavierbussiere+testing@gmail.com"

	e := email.NewEmail()
	e.From = fmt.Sprintf("Parfait Saucier <%s>", from)
	e.To = []string{to}
	e.Subject = "[Test] New Music Friday"
	e.HTML = body.Bytes()
	e.AttachFile("logo/spotify/spotify-icon.png")
	e.AttachFile("logo/tidal/tidal-icon.png")

	e.Send(fmt.Sprintf("%s:%d", smtpHost, smtpPort), smtp.PlainAuth("", from, password, smtpHost))

	if err != nil {
		return err
	}
	log.Info().Msgf("✨ Email sent successfully to %s ✨", to)
	return nil
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	viper.SetConfigFile("configs/config.yaml")

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("fatal error config file")
	}

	// Tidal Authorization
	authKey, err := tidal.GetAuthorization()
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	// Spotify Authorization
	err = spotify.GetAccessToken()
	if err != nil {
		log.Fatal().Err(err).Msg("Error while making a request for an access token with Spotify")
	}

	var albums []album.Album

	//genres := []string{"german soundtrack"}
	genres := []string{"german indie", "phonk", "wonky", "rap", "indietronica", "rock", "new wave", "electro", "art pop", "hip hop", "indie soul"}

	for _, genre := range genres {
		err = genreScraper(genre, &albums)
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}

	albums = album.RemoveCopies(albums)

	for i := range albums {
		log.Debug().Str("album_name", albums[i].AlbumName).Str("artist_name", albums[i].ArtistName).Msg("")
	}

	for i := range albums {
		err = tidal.SearchAlbum(&albums[i], authKey)
		if err != nil {
			log.Error().Err(err).Msgf("error encountered while searching the album '%s' on Tidal", albums[i].AlbumName)
		}
		err = spotify.SearchAlbum(&albums[i])
		if err != nil {
			log.Error().Err(err).Msgf("error encountered while searching the album '%s' on Spotify", albums[i].AlbumName)
		}
		err = spotify.GetArtists(&albums[i])
		if err != nil {
			log.Error().Err(err).Msgf("error encountered while getting the artist '%s' info on Spotify", albums[i].ArtistName)
		}
	}
	err = spotify.GetAlbums(&albums)
	if err != nil {
		log.Error().Err(err).Msgf("error encountered during spotify.GetAlbums")
	}

	albums = album.RankByPopularity(albums)

	err = emailSender(&albums)
	if err != nil {
		log.Error().Err(err).Msg("error encountered during sending email")
	}
}
