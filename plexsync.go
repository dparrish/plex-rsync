package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"

	plex "github.com/jrudio/go-plex-client"
	"github.com/olekukonko/tablewriter"
)

var (
	destAddr = flag.String("dest_host", "passport", "Destination IP")
	destPath = flag.String("dest_path", "/DataVolume/plexsync/", "Destination Path")

	playlistId    = flag.Int("playlist", 0, "Playlist to synchronize")
	syncOnDeck    = flag.Bool("ondeck", false, "Sync On-Deck items")
	search        = flag.String("search", "", "Sync all matching search items")
	unwatchedOnly = flag.Bool("unwatched_only", false, "Sync only unwatched items")

	keyRe = regexp.MustCompile(`^/library/metadata/([0-9]+)(?:/children)?$`)
)

func toJson(data interface{}) string {
	t, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	return string(t)
}

func main() {
	flag.Parse()
	var syncFiles []string

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Episode", "Name", "Filename"})

	conn, err := plex.New("http://192.168.1.7:32400", "")
	if err != nil {
		log.Fatalf("Error connecting to plex: %v", err)
	}

	_, err = conn.Test()
	if err != nil {
		log.Fatalf("Error connecting to plex: %v", err)
	}

	if *syncOnDeck {
		ondeck, err := conn.GetOnDeck()
		if err != nil {
			log.Fatalf("Error getting on deck: %v", err)
		}
		for _, video := range ondeck.MediaContainer.Metadata {
			if *unwatchedOnly && video.ViewCount > 0 {
				continue
			}
			var files []string
			for _, media := range video.Media {
				for _, part := range media.Part {
					if part.File != "" {
						files = append(files, part.File)
					}
				}
			}
			if len(files) == 0 {
				continue
			}
			syncFiles = append(syncFiles, files[0])

			switch video.Type {
			case "episode":
				table.Append([]string{
					fmt.Sprintf("S%02dE%02d", video.ParentIndex, video.Index),
					video.Title,
					files[0],
				})
			case "movie":
				table.Append([]string{
					"",
					video.Title,
					files[0],
				})
			}
		}
	}

	if *playlistId != 0 {
		playlist, err := conn.GetPlaylist(*playlistId)
		if err != nil {
			log.Fatalf("Error getting playlist: %v", err)
		}
		for _, video := range playlist.MediaContainer.Metadata {
			if *unwatchedOnly && video.ViewCount > 0 {
				continue
			}
			var files []string
			for _, media := range video.Media {
				for _, part := range media.Part {
					if part.File != "" {
						files = append(files, part.File)
					}
				}
			}
			if len(files) == 0 {
				continue
			}
			syncFiles = append(syncFiles, files[0])

			switch video.Type {
			case "episode":
				table.Append([]string{
					fmt.Sprintf("S%02dE%02d", video.ParentIndex, video.Index),
					video.Title,
					files[0],
				})
			case "movie":
				table.Append([]string{
					"",
					video.Title,
					files[0],
				})
			}
		}
	}

	if *search != "" {
		shows, err := conn.Search(*search)
		if err != nil {
			log.Fatalf("Error searching on plex: %v", err)
		}
		for _, show := range shows.MediaContainer.Metadata {
			if show.Type != "show" {
				continue
			}

			seasons, err := conn.GetMetadataChildren(show.RatingKey)
			if err != nil {
				log.Fatalf("Error getting seasons: %v", err)
			}

			for _, season := range seasons.MediaContainer.Metadata {
				episodes, err := conn.GetEpisodes(season.RatingKey)
				if err != nil {
					log.Fatalf("Error getting episodes: %v", err)
				}
				for _, video := range episodes.MediaContainer.Metadata {
					if video.Type != "episode" {
						continue
					}
					if *unwatchedOnly && video.ViewCount > 0 {
						continue
					}
					var files []string
					for _, media := range video.Media {
						for _, part := range media.Part {
							if part.File != "" {
								files = append(files, part.File)
							}
						}
					}
					if len(files) == 0 {
						continue
					}
					table.Append([]string{
						fmt.Sprintf("S%02dE%02d", season.Index, video.Index),
						video.Title,
						files[0],
					})
					syncFiles = append(syncFiles, files[0])
				}
			}
		}
	}

	if len(syncFiles) == 0 {
		fmt.Fprintf(os.Stderr, "Nothing to copy\n")
		os.Exit(1)
	}
	table.Render()

	cmdline := []string{
		"rsync",
		"-essh",
		"-av",
		"--progress",
		"--inplace",
	}
	cmdline = append(cmdline, syncFiles...)

	cmdline = append(cmdline, fmt.Sprintf("%s:%s", *destAddr, *destPath))
	fmt.Println(cmdline)
	cmd := exec.Command(cmdline[0], cmdline[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}
