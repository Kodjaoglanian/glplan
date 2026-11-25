package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// latestReleaseURL is a variable so tests can point at a local server.
var latestReleaseURL = "https://api.github.com/repos/Kodjaoglanian/glplan/releases/latest"

type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// compareVersions returns -1, 0 or 1 comparing two semver-ish versions.
func compareVersions(a, b string) int {
	parse := func(v string) [3]int {
		v = strings.TrimPrefix(strings.TrimSpace(v), "v")
		if i := strings.IndexAny(v, "-+"); i >= 0 {
			v = v[:i]
		}
		var out [3]int
		for i, part := range strings.Split(v, ".") {
			if i > 2 {
				break
			}
			out[i], _ = strconv.Atoi(part)
		}
		return out
	}
	pa, pb := parse(a), parse(b)
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

func updateAssetName(tag, goos, goarch string) string {
	name := fmt.Sprintf("glplan-%s-%s-%s", tag, goos, goarch)
	if goos == "windows" {
		return name + ".zip"
	}
	return name + ".tar.gz"
}

func httpGet(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "glplan-update")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 128<<20))
}

// extractBinary pulls the glplan binary out of a .tar.gz or .zip archive.
func extractBinary(archive []byte, name string) ([]byte, error) {
	if strings.HasSuffix(name, ".tar.gz") {
		zr, err := gzip.NewReader(bytes.NewReader(archive))
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		tr := tar.NewReader(zr)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if filepath.Base(hdr.Name) == "glplan" && hdr.Typeflag == tar.TypeReg {
				return io.ReadAll(tr)
			}
		}
		return nil, fmt.Errorf("binário glplan não encontrado em %s", name)
	}
	if strings.HasSuffix(name, ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if filepath.Base(f.Name) == "glplan.exe" {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(rc)
			}
		}
		return nil, fmt.Errorf("glplan.exe não encontrado em %s", name)
	}
	return nil, fmt.Errorf("formato de arquivo desconhecido: %s", name)
}

func verifySHA256(data, checksumLine []byte) error {
	fields := strings.Fields(string(checksumLine))
	if len(fields) == 0 {
		return fmt.Errorf("arquivo de checksum vazio")
	}
	sum := sha256.Sum256(data)
	if !strings.EqualFold(fields[0], hex.EncodeToString(sum[:])) {
		return fmt.Errorf("checksum não confere")
	}
	return nil
}

// replaceExecutable atomically swaps the running binary where possible.
func replaceExecutable(binary []byte) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, "glplan-update-*")
	if err != nil {
		return "", fmt.Errorf("sem permissão de escrita em %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(binary); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return "", err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		os.Remove(tmpName)
		return "", err
	}
	if err := os.Rename(tmpName, exe); err != nil {
		os.Remove(tmpName)
		if runtime.GOOS == "windows" {
			return "", fmt.Errorf("não foi possível substituir o executável em uso; reinstale com install.ps1")
		}
		return "", err
	}
	return exe, nil
}

// replaceBinaryFn is a variable so tests can stub the install step.
var replaceBinaryFn = replaceExecutable

// selfUpdate checks the latest release and updates the binary when newer.
// It returns a human-readable result message.
func selfUpdate(current string) (string, error) {
	body, err := httpGet(latestReleaseURL)
	if err != nil {
		return "", fmt.Errorf("não foi possível consultar releases: %w", err)
	}
	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", fmt.Errorf("resposta inválida do GitHub: %w", err)
	}
	if rel.TagName == "" {
		return "", fmt.Errorf("nenhuma release encontrada")
	}
	if compareVersions(current, rel.TagName) >= 0 {
		return fmt.Sprintf("glplan já está atualizado (%s)", current), nil
	}

	assetName := updateAssetName(rel.TagName, runtime.GOOS, runtime.GOARCH)
	var assetURL, sumURL string
	for _, a := range rel.Assets {
		if a.Name == assetName {
			assetURL = a.BrowserDownloadURL
		}
		if a.Name == assetName+".sha256" {
			sumURL = a.BrowserDownloadURL
		}
	}
	if assetURL == "" || sumURL == "" {
		return "", fmt.Errorf("asset %s não encontrado na release %s", assetName, rel.TagName)
	}

	archive, err := httpGet(assetURL)
	if err != nil {
		return "", fmt.Errorf("falha ao baixar %s: %w", assetName, err)
	}
	sum, err := httpGet(sumURL)
	if err != nil {
		return "", fmt.Errorf("falha ao baixar checksum: %w", err)
	}
	if err := verifySHA256(archive, sum); err != nil {
		return "", err
	}
	binary, err := extractBinary(archive, assetName)
	if err != nil {
		return "", err
	}
	exe, err := replaceBinaryFn(binary)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("atualizado %s → %s (%s)", current, rel.TagName, exe), nil
}

func runUpdate() {
	if version == "dev" {
		fmt.Fprintln(os.Stderr, "build de desenvolvimento: a atualização substituiria este binário pelo release publicado")
	}
	msg, err := selfUpdate(version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "update:", err)
		os.Exit(1)
	}
	fmt.Println(msg)
}
