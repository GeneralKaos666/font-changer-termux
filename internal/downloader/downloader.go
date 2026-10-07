// Package downloader fetches Nerd Fonts into the ~/.termux/fonts library.
//
// URLs mirror ~/.termux/fonts.sh (NF_VERSION=v3.2.1). Downloads go to a
// .part temp file in the library directory, then atomically rename into
// place; partial files are removed on any error.
package downloader

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"termux-fonts-go/internal/paths"
	"termux-fonts-go/internal/validate"
)

const nfBase = "https://github.com/ryanoasis/nerd-fonts/raw/v3.2.1/patched-fonts"

// NerdFonts maps display names to download URLs.
var NerdFonts = map[string]string{
	"JetBrainsMono-Light":   nfBase + "/JetBrainsMono/Ligatures/Light/JetBrainsMonoNerdFont-Light.ttf",
	"JetBrainsMono-Regular": nfBase + "/JetBrainsMono/Ligatures/Regular/JetBrainsMonoNerdFont-Regular.ttf",
	"Hack-Regular":          nfBase + "/Hack/Regular/HackNerdFont-Regular.ttf",
	"FiraCode-Regular":      nfBase + "/FiraCode/Regular/FiraCodeNerdFont-Regular.ttf",
	"SourceCodePro-Regular": nfBase + "/SourceCodePro/SauceCodeProNerdFont-Regular.ttf",
	"IosevkaTerm-Regular":   nfBase + "/IosevkaTerm/IosevkaTermNerdFont-Regular.ttf",
	"Mononoki-Regular":      nfBase + "/Mononoki/Regular/MononokiNerdFont-Regular.ttf",
	"Terminus-Regular":      nfBase + "/Terminus/TerminessNerdFont-Regular.ttf",
	"CascadiaCode-Regular":  nfBase + "/CascadiaCode/Regular/CaskaydiaCoveNerdFont-Regular.ttf",
	"IBMPlexMono-Regular":   nfBase + "/IBMPlexMono/Mono/BlexMonoNerdFontMono-Regular.ttf",
	"AnonymousPro-Regular":  nfBase + "/AnonymousPro/Regular/AnonymiceProNerdFont-Regular.ttf",
}

// remoteSize returns the remote Content-Length, or nil when unknown.
// The size check is best-effort; the download itself decides.
func remoteSize(rawURL string) *int64 {
	resp, err := http.Head(rawURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	if resp.ContentLength < 0 {
		return nil
	}
	n := resp.ContentLength
	return &n
}

func destFor(rawURL, name string) string {
	filename := ""
	if u, err := url.Parse(rawURL); err == nil {
		if un, err := url.PathUnescape(path.Base(u.Path)); err == nil {
			filename = un
		}
	}
	if filename == "" || filename == "/" || filename == "." {
		filename = name + ".ttf"
	}
	return filepath.Join(paths.FontsDir(), filename)
}

// Fetch downloads Nerd Font name into the library and returns its path.
//
// It skips the download when the file already exists with the same size as
// the remote (unless force is true). Unknown names and invalid downloads
// return an error.
func Fetch(name string, force bool) (string, error) {
	rawURL, ok := NerdFonts[name]
	if !ok {
		choices := make([]string, 0, len(NerdFonts))
		for k := range NerdFonts {
			choices = append(choices, k)
		}
		sort.Strings(choices)
		return "", fmt.Errorf("unknown font %q; choose from: %s", name, strings.Join(choices, ", "))
	}
	destDir := paths.FontsDir()
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	dest := destFor(rawURL, name)
	if st, err := os.Stat(dest); err == nil && !force {
		if remote := remoteSize(rawURL); remote != nil && *remote == st.Size() {
			return dest, nil
		}
	}
	stem := strings.TrimSuffix(filepath.Base(dest), filepath.Ext(dest))
	tmp, err := os.CreateTemp(destDir, stem+".*.part")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	okDownload := false
	defer func() {
		tmp.Close()
		if !okDownload {
			os.Remove(tmpName)
		}
	}()
	resp, err := http.Get(rawURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download of %q failed: %s", name, resp.Status)
	}
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if valid, reason := validate.IsValidFont(tmpName); !valid {
		return "", fmt.Errorf("downloaded file for %q is not a valid font: %s", name, reason)
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return "", err
	}
	okDownload = true
	return dest, nil
}
