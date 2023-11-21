package aggregator

import "sort"

type Album struct {
	AlbumArt         string
	ArtistName       string
	AlbumName        string
	Genre            string
	TidalURL         string
	ArtistPopularity int
}

func remove(a []Album, i int) []Album {
	a[i] = a[len(a)-1]
	return a[:len(a)-1]
}

func RemoveCopies(albums []Album) []Album {
	for i := range albums {
		for j := i + 1; j < len(albums); j++ {
			if albums[i].AlbumName == albums[j].AlbumName && albums[i].ArtistName == albums[j].ArtistName {
				albums[i].Genre = albums[i].Genre + ", " + albums[j].Genre
				albums = remove(albums, j)
			}
		}
	}
	return albums
}

func RankByPopularity(albums []Album) []Album {
	sort.Slice(albums, func(i, j int) bool {
		return albums[j].ArtistPopularity < albums[i].ArtistPopularity
	})
	return albums
}
