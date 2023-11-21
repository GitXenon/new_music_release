package main

import (
	"bytes"
	"fmt"
	"net/smtp"
	"newmusicrelease/aggregator"
	"newmusicrelease/spotify"
	"newmusicrelease/tidal"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
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

func genreScraper(genre string, albums *[]aggregator.Album) error {
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
			newAlbum := aggregator.Album{
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

func emailSender(albums *[]aggregator.Album) error {
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
	err = spotify.RequestAccessTokenSpotify()
	if err != nil {
		log.Fatal().Err(err).Msg("Error while making a request for an access token with Spotify")
	}

	var albums []aggregator.Album

	genres := []string{"electro"}
	//genres := []string{"german indie", "art pop", "phonk", "wonky", "rap", "indietronica", "rock", "new wave", "electro", "art pop", "hip hop", "indie soul"}

	for _, genre := range genres {
		err = genreScraper(genre, &albums)
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}

	albums = aggregator.RemoveCopies(albums)

	for i := range albums {
		err = tidal.GetTidalURL(&albums[i], authKey)
		if err != nil {
			log.Error().Err(err).Msg("error ecountered while getting the Tidal link")
		}
		err = spotify.GetPopularityArtistSpotify(&albums[i])
		if err != nil {
			log.Error().Err(err).Msg("error ecountered while getting the Spotify's artist popularity")
		}
		log.Info().Interface("album", albums[i]).Msg("")
	}

	albums = aggregator.RankByPopularity(albums)

	err = emailSender(&albums)
	if err != nil {
		log.Error().Err(err).Msg("error ecountered during sending email")
	}
}
