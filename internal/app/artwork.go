package app

import (
	ampapi "amdl/internal/amp-api"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (r *Runner) saveAlbumArtistArtwork(root, albumArtistFolder string, album *ampapi.AlbumRespData) {
	cfg := r.Config.Metadata.Artwork
	artists := album.Relationships.Artists.Data
	seen := make(map[string]bool)
	usedPaths := make(map[string]bool)
	for _, artist := range artists {
		id, artworkURL := artist.ID, artist.Attributes.Artwork.Url
		if id == "" || strings.Trim(id, "0123456789") != "" || artworkURL == "" || seen[id] {
			continue
		}
		seen[id] = true

		artistName := r.LimitString(artist.Attributes.Name)
		name := strings.Trim(forbiddenNames.ReplaceAllString(artistName, "_"), " .")
		if name == "" {
			name = id
		}
		dir, file := albumArtistFolder, "folder"
		switch {
		case cfg.CentralizeArtistImages:
			dir, file = cfg.ArtistPath, name
			if dir == "" {
				dir = filepath.Join(root, "ArtistImageFolder")
			}
		case cfg.ArtistPath != "":
			dir = filepath.Join(cfg.ArtistPath, id)
		case len(artists) > 1:
			folder := strings.NewReplacer("{ArtistId}", id, "{ArtistName}", name, "{UrlArtistName}", name).
				Replace(r.Config.Paths.ArtistFolder)
			folder = strings.Trim(forbiddenNames.ReplaceAllString(folder, "_"), " .")
			if folder == "" {
				folder = id
			}
			dir = filepath.Join(root, folder)
		}
		key := strings.ToLower(filepath.Join(dir, file))
		if usedPaths[key] {
			file += " [" + id + "]"
		}
		usedPaths[strings.ToLower(filepath.Join(dir, file))] = true
		if err := createDirectory(dir); err != nil {
			fmt.Printf("Failed to create artist artwork folder for %s: %v\n", id, err)
			continue
		}
		if _, err := r.writeCover(dir, file, artworkURL); err != nil {
			fmt.Printf("Failed to write artist artwork for %s: %v\n", id, err)
		}
	}
}

func (r *Runner) saveAnimatedArtwork(dir string, squareVideoURL string, tallVideoURL string) {
	if squareVideoURL != "" {
		if err := r.downloadAnimatedArtwork(dir, "square", squareVideoURL); err != nil {
			fmt.Printf("no motion video square: %v\n", err)
		}
		if r.Config.Metadata.Artwork.EmbyAnimated {
			cmd := exec.Command("ffmpeg", "-i", filepath.Join(dir, "square_animated_artwork.mp4"), "-vf", "scale=440:-1", "-r", "24", "-f", "gif", filepath.Join(dir, "folder.jpg"))
			if err := cmd.Run(); err != nil {
				fmt.Printf("animated artwork square to gif err: %v\n", err)
			}
		}
	}
	if tallVideoURL != "" {
		if err := r.downloadAnimatedArtwork(dir, "tall", tallVideoURL); err != nil {
			fmt.Printf("no motion video tall: %v\n", err)
		}
	}
}

func (r *Runner) downloadAnimatedArtwork(dir string, orientation string, videoURL string) error {
	videoURL, err := r.extractVideo(videoURL)
	if err != nil {
		return err
	}
	output := filepath.Join(dir, orientation+"_animated_artwork.mp4")
	exists, err := fileExists(output)
	if err != nil {
		return fmt.Errorf("check existing artwork: %w", err)
	}
	if exists {
		fmt.Printf("Animated artwork %s already exists locally.\n", orientation)
		return nil
	}
	fmt.Printf("Animation Artwork %s Downloading...\n", orientation)
	cmd := exec.Command("ffmpeg", "-loglevel", "quiet", "-y", "-i", videoURL, "-c", "copy", output)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("download animated artwork: %w", err)
	}
	fmt.Printf("Animation Artwork %s Downloaded\n", orientation)
	return nil
}

func sanitizeFolderName(name string) string {
	if name == "" {
		return ""
	}
	if name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	return forbiddenNames.ReplaceAllString(name, "_")
}

func (r *Runner) prepareCollectionFolder(base string, displayName string) (string, error) {
	if strings.HasSuffix(displayName, ".") {
		displayName = displayName[:len(displayName)-1]
	}
	displayName = strings.TrimSpace(displayName)
	folderPath := filepath.Join(base, sanitizeFolderName(displayName))
	if err := createDirectory(folderPath); err != nil {
		return "", err
	}
	return folderPath, nil
}

func (r *Runner) saveM3UPlaylist(dir string, name string, startIdx int) {
	if !r.Flags.SaveM3U8 {
		return
	}
	if len(r.State.AddedTracks) > startIdx {
		if err := r.writeM3UPlaylist(dir, name, r.State.AddedTracks[startIdx:]); err != nil {
			fmt.Printf("Failed to write M3U8 playlist: %v\n", err)
		} else {
			fmt.Println("M3U8 playlist saved.")
		}
	}
}

func createDirectory(path string) error {
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return fmt.Errorf("create directory %s: %w", path, err)
	}
	return nil
}
