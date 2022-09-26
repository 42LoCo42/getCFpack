package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	resty "github.com/go-resty/resty/v2"
)

const (
	API_URL   = "https://api.curseforge.com"
	CDN_URL   = "https://edge.forgecdn.net/files"
	OVERRIDES = "overrides"
)

type Ctx struct {
	*resty.Client
}

type File struct {
	ID          int    `json:"id"`
	DownloadURL string `json:"downloadURL"`
	FileName    string `json:"fileName"`
}

func (file File) URL() string {
	id := strconv.Itoa(file.ID)

	if file.DownloadURL == "" {
		return strings.Join([]string{
			CDN_URL,
			id[:4],
			strings.TrimPrefix(id[4:], "0"),
			file.FileName,
		}, "/")
	} else {
		return file.DownloadURL
	}
}

type Manifest struct {
	Minecraft struct {
		Version    string `json:"version"`
		ModLoaders []struct {
			ID string `json:"id"`
		} `json:"modLoaders"`
	} `json:"minecraft"`
	Files []struct {
		ProjectID int `json:"projectID"`
		FileID    int `json:"fileID"`
	} `json:"files"`
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <pack ID>\n", os.Args[0])
	}

	packID := os.Args[1]

	key, err := ioutil.ReadFile("key")
	if err != nil {
		panic(err)
	}

	ctx := Ctx{resty.New()}
	ctx.
		SetHeader("Accept", "application/json").
		SetHeader("x-api-key", strings.TrimSpace(string(key)))

	// get pack URL
	packURL, err := ctx.getProjectURL(packID)
	if err != nil {
		panic(err)
	}
	log.Print("Pack URL is ", packURL)

	// download pack
	file := path.Base(packURL)
	if err := downloadFileTo(packURL, file); err != nil {
		panic(err)
	}

	// extract pack
	packDir := strings.TrimSuffix(file, ".zip")
	log.Print("Extracting to directory ", packDir)
	if err := unzip(file, packDir); err != nil {
		panic(err)
	}
	os.Remove(file)

	// read manifest
	rawManifest, err := ioutil.ReadFile(path.Join(packDir, "manifest.json"))
	if err != nil {
		panic(err)
	}

	var manifest Manifest
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		panic(err)
	}

	// clean up pack structure
	entries, err := os.ReadDir(packDir)
	if err != nil {
		panic(err)
	}

	for _, entry := range entries {
		if entry.Name() != OVERRIDES {
			os.Remove(path.Join(packDir, entry.Name()))
		}
	}

	entries, err = os.ReadDir(path.Join(packDir, OVERRIDES))

	if err == nil {
		for _, entry := range entries {
			os.Rename(
				path.Join(packDir, OVERRIDES, entry.Name()),
				path.Join(packDir, entry.Name()),
			)
		}
	}

	os.Remove(path.Join(packDir, OVERRIDES))

	// download mods
	failures := []string{}
	modDir := path.Join(packDir, "mods")
	if err := os.MkdirAll(modDir, 0755); err != nil {
		panic(err)
	}

	for _, mod := range manifest.Files {
		url, err := ctx.getFileURL(strconv.Itoa(mod.ProjectID), strconv.Itoa(mod.FileID))
		if err != nil || url == "" {
			log.Printf("Could not get URL for file %d/%d", mod.ProjectID, mod.FileID)
			failures = append(failures, fmt.Sprintf("%d/%d", mod.ProjectID, mod.FileID))
			continue
		}

		log.Print("Downloading ", url)
		downloadFileTo(url, path.Join(modDir, path.Base(url)))
	}

	// print final information
	log.Print("\nMinecraft version ", manifest.Minecraft.Version)
	for _, loader := range manifest.Minecraft.ModLoaders {
		log.Print("ModLoader version ", loader.ID)
	}
	log.Printf("%d mods", len(manifest.Files))

	if len(failures) > 0 {
		log.Print("Failed IDs:")
		for _, failure := range failures {
			log.Print("\t", failure)
		}
	}
}

func (ctx Ctx) getProjectURL(projectID string) (string, error) {
	type res struct {
		Data []File `json:"data"`
	}

	resp, err := ctx.R().
		SetResult(res{}).
		Get(fmt.Sprintf("%s/v1/mods/%s/files", API_URL, projectID))
	if err != nil {
		return "", err
	}

	data := resp.Result().(*res).Data
	if len(data) < 1 {
		return "", errors.New("no data returned")
	}

	return data[0].URL(), nil
}

func (ctx Ctx) getFileURL(projectID, fileID string) (string, error) {
	type res struct {
		File File `json:"data"`
	}

	resp, err := ctx.R().
		SetResult(res{}).
		Get(fmt.Sprintf("%s/v1/mods/%s/files/%s", API_URL, projectID, fileID))
	if err != nil {
		return "", err
	}

	return resp.Result().(*res).File.URL(), nil
}

func downloadFileTo(url, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func unzip(source, dest string) error {
	read, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer read.Close()

	for _, file := range read.File {
		if file.Mode().IsDir() {
			continue
		}

		open, err := file.Open()
		if err != nil {
			return err
		}

		name := path.Join(dest, file.Name)
		os.MkdirAll(path.Dir(name), 0755)

		create, err := os.Create(name)
		if err != nil {
			return err
		}
		defer create.Close()

		create.ReadFrom(open)
	}
	return nil
}
