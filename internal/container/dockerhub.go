package container

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type AuthResponse struct {
	Token string `json:"token"`
}

type ManifestResponse struct {
	Layers []struct {
		Digest string `json:"digest"`
	} `json:"layers"`
}

// for reading architecture index
type ManifestIndex struct {
	Manifests []struct {
		Digest   string `json:"digest"`
		Platform struct {
			Architecture string `json:"architecture"`
			OS           string `json:"os"`
		} `json:"platform"`
	} `json:"manifests"`
}

func fetchManifest(url, token, acceptHeader string) (*http.Response, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", acceptHeader)

	client := &http.Client{}
	return client.Do(req)
}

func Pull(image string) {
	parts := strings.Split(image, ":")
	repo := parts[0]
	tag := "latest"
	if len(parts) == 2 {
		tag = parts[1]
	}
	if !strings.Contains(repo, "/") {
		repo = "library/" + repo
	}

	fmt.Printf("Pulling %s:%s from Docker Hub...\n", repo, tag) // Correct

	authUrl := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", repo)
	res, err := http.Get(authUrl)
	if err != nil {
		fmt.Printf("Auth request failed: %v\n", err)
		return
	}
	defer res.Body.Close()

	var auth AuthResponse
	json.NewDecoder(res.Body).Decode(&auth)

	// get image manifest
	manifestUrl := fmt.Sprintf("https://registry-1.docker.io/v2/%s/manifests/%s", repo, tag)
	// either accept a Manifest List (Index) OR a standard v2 Manifest
	acceptHeader := "application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.docker.distribution.manifest.v2+json"

	res, err = fetchManifest(manifestUrl, auth.Token, acceptHeader)
	if err != nil || res.StatusCode != 200 {
		fmt.Printf("Failed to fetch manifest. Status: %d\n", res.StatusCode)
		return
	}
	defer res.Body.Close()

	bytes, _ := io.ReadAll(res.Body)

	var manifest ManifestResponse
	var index ManifestIndex
	json.Unmarshal(bytes, &index)

	// if json contains manifests, find our architecture in that manifests
	if len(index.Manifests) > 0 {
		fmt.Printf("Multi-architecture index detected. Searching for %s/linux...\n", runtime.GOARCH)

		targetDigest := ""
		for _, m := range index.Manifests {
			if m.Platform.Architecture == runtime.GOARCH && m.Platform.OS == "linux" {
				targetDigest = m.Digest
				break
			}
		}

		if targetDigest == "" {
			fmt.Printf("Error: No image found for architecture %s/linux\n", runtime.GOARCH)
			return
		}

		manifestUrl = fmt.Sprintf("https://registry-1.docker.io/v2/%s/manifests/%s", repo, targetDigest)
		res, _ = fetchManifest(manifestUrl, auth.Token, "application/vnd.docker.distribution.manifest.v2+json")
		defer res.Body.Close()

		bytes, _ = io.ReadAll(res.Body)
	}

	json.Unmarshal(bytes, &manifest)

	if len(manifest.Layers) == 0 {
		fmt.Println("No layers found after parsing!")
		return
	}

	// download and extract layers and put in target directory
	targetDir := filepath.Join("/var/lib/o1/images", strings.ReplaceAll(repo, "/", "_")+"_"+tag)
	os.RemoveAll(targetDir)
	os.MkdirAll(targetDir, 0755)

	client := &http.Client{}

	for i, layer := range manifest.Layers {
		fmt.Printf("Downloading layer %d/%d: %s\n", i+1, len(manifest.Layers), layer.Digest[:12])

		blobUrl := fmt.Sprintf("https://registry-1.docker.io/v2/%s/blobs/%s", repo, layer.Digest)
		req, err := http.NewRequest("GET", blobUrl, nil)
		if err != nil {
			fmt.Printf("Failed to make http request: %v\n", err)
			return
		}
		req.Header.Set("Authorization", "Bearer "+auth.Token)

		blob, err := client.Do(req)
		if err != nil {
			fmt.Printf("Failed to download layer: %v\n", err)
			return
		}

		// save the compressed tarball to a temporary file
		tmpFile := filepath.Join("/tmp", layer.Digest+".tar.gz")
		out, _ := os.Create(tmpFile)
		io.Copy(out, blob.Body)
		out.Close()
		blob.Body.Close()

		// extract tarball to target directory
		exec.Command("tar", "-xzf", tmpFile, "-C", targetDir).Run()
		os.Remove(tmpFile)
	}
	fmt.Printf("Successfully pulled %s into %s\n", image, targetDir)
}
