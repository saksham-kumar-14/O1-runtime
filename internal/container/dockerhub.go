package container

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

func Pull(image string) {
	parts := strings.Split(image, "/")
	repo := parts[0]
	tag := "latest"
	if len(parts) == 2 {
		tag = parts[1]
	}
	if !strings.Contains(repo, "/") {
		repo = "library/" + repo
	}

	fmt.Sprint("Pulling %s:%s form dockerhub", repo, tag)

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
	req, _ := http.NewRequest("GET", manifestUrl, nil)
	req.Header.Set("Authorization", "Bearer "+auth.Token)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json") // Explicitly request the standard V2 manifest

	client := &http.Client{}
	res, err = client.Do(req)
	if err != nil || res.StatusCode != 200 {
		fmt.Printf("Failed to fetch manifest. Status: %d\n", res.StatusCode)
		return
	}
	defer res.Body.Close()

	var manifest ManifestResponse
	json.NewDecoder(res.Body).Decode(&manifest)

	if len(manifest.Layers) == 0 {
		fmt.Println("No layers found for this image. It might be an architecture index.")
		return
	}

	// download and extract layers and put in target directory
	targetDir := filepath.Join("/var/lib/o1/images", strings.ReplaceAll(repo, "/", "_")+"_"+tag)
	os.RemoveAll(targetDir)
	os.MkdirAll(targetDir, 0755)

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
