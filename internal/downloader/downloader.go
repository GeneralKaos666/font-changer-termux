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

// NerdFonts maps display names to download URLs: one Regular weight for
// every family in the pinned release, plus the original Light entry.
// Symbols-only and proportional (Propo) variants are intentionally
// excluded — they are not usable as a terminal text font.
var NerdFonts = map[string]string{
	"3270-Regular":                  nfBase + "/3270/Regular/3270NerdFont-Regular.ttf",
	"Agave-Regular":                 nfBase + "/Agave/AgaveNerdFont-Regular.ttf",
	"AnonymousPro-Regular":          nfBase + "/AnonymousPro/Regular/AnonymiceProNerdFont-Regular.ttf",
	"Arimo-Regular":                 nfBase + "/Arimo/Regular/ArimoNerdFont-Regular.ttf",
	"BigBlueTerminal-Regular":       nfBase + "/BigBlueTerminal/BigBlueTerm437NerdFont-Regular.ttf",
	"BitstreamVeraSansMono-Regular": nfBase + "/BitstreamVeraSansMono/Regular/BitstromWeraNerdFont-Regular.ttf",
	"CascadiaCode-Regular":          nfBase + "/CascadiaCode/Regular/CaskaydiaCoveNerdFont-Regular.ttf",
	"Cousine-Regular":               nfBase + "/Cousine/Regular/CousineNerdFont-Regular.ttf",
	"DaddyTimeMono-Regular":         nfBase + "/DaddyTimeMono/DaddyTimeMonoNerdFont-Regular.ttf",
	"DejaVuSansMono-Regular":        nfBase + "/DejaVuSansMono/Regular/DejaVuSansMNerdFont-Regular.ttf",
	"EnvyCodeR-Regular":             nfBase + "/EnvyCodeR/EnvyCodeRNerdFont-Regular.ttf",
	"FantasqueSansMono-Regular":     nfBase + "/FantasqueSansMono/Regular/FantasqueSansMNerdFont-Regular.ttf",
	"FiraCode-Regular":              nfBase + "/FiraCode/Regular/FiraCodeNerdFont-Regular.ttf",
	"GoMono-Regular":                nfBase + "/Go-Mono/Regular/GoMonoNerdFont-Regular.ttf",
	"Gohu-Regular":                  nfBase + "/Gohu/uni-14/GohuFontuni14NerdFont-Regular.ttf",
	"Hack-Regular":                  nfBase + "/Hack/Regular/HackNerdFont-Regular.ttf",
	"HeavyData-Regular":             nfBase + "/HeavyData/HeavyDataNerdFont-Regular.ttf",
	"IBMPlexMono-Regular":           nfBase + "/IBMPlexMono/Mono/BlexMonoNerdFontMono-Regular.ttf",
	"Inconsolata-Regular":           nfBase + "/Inconsolata/InconsolataNerdFont-Regular.ttf",
	"InconsolataGo-Regular":         nfBase + "/InconsolataGo/Regular/InconsolataGoNerdFont-Regular.ttf",
	"InconsolataLGC-Regular":        nfBase + "/InconsolataLGC/InconsolataLGCNerdFont-Regular.ttf",
	"Iosevka-Regular":               nfBase + "/Iosevka/IosevkaNerdFont-Regular.ttf",
	"IosevkaTerm-Regular":           nfBase + "/IosevkaTerm/IosevkaTermNerdFont-Regular.ttf",
	"JetBrainsMono-Light":           nfBase + "/JetBrainsMono/Ligatures/Light/JetBrainsMonoNerdFont-Light.ttf",
	"JetBrainsMono-Regular":         nfBase + "/JetBrainsMono/Ligatures/Regular/JetBrainsMonoNerdFont-Regular.ttf",
	"Lekton-Regular":                nfBase + "/Lekton/Regular/LektonNerdFont-Regular.ttf",
	"LiberationMono-Regular":        nfBase + "/LiberationMono/LiterationMonoNerdFont-Regular.ttf",
	"Lilex-Regular":                 nfBase + "/Lilex/LilexNerdFont-Regular.ttf",
	"MPlus1-Regular":                nfBase + "/MPlus/M_Plus_1/M+1NerdFont-Regular.ttf",
	"MesloLG-Regular":               nfBase + "/Meslo/L/Regular/MesloLGLNerdFont-Regular.ttf",
	"Monofur-Regular":               nfBase + "/Monofur/Regular/MonofurNerdFont-Regular.ttf",
	"Monoid-Regular":                nfBase + "/Monoid/Regular/MonoidNerdFont-Regular.ttf",
	"Mononoki-Regular":              nfBase + "/Mononoki/Regular/MononokiNerdFont-Regular.ttf",
	"NotoSansMono-Regular":          nfBase + "/Noto/Sans-Mono/NotoSansMNerdFont-Regular.ttf",
	"ProFont-Regular":               nfBase + "/ProFont/profontiix/ProFontIIxNerdFont-Regular.ttf",
	"ProggyClean-Regular":           nfBase + "/ProggyClean/Regular/ProggyCleanNerdFont-Regular.ttf",
	"RobotoMono-Regular":            nfBase + "/RobotoMono/Regular/RobotoMonoNerdFont-Regular.ttf",
	"ShareTechMono-Regular":         nfBase + "/ShareTechMono/ShureTechMonoNerdFont-Regular.ttf",
	"SourceCodePro-Regular":         nfBase + "/SourceCodePro/SauceCodeProNerdFont-Regular.ttf",
	"SpaceMono-Regular":             nfBase + "/SpaceMono/Regular/SpaceMonoNerdFont-Regular.ttf",
	"Terminus-Regular":              nfBase + "/Terminus/TerminessNerdFont-Regular.ttf",
	"Tinos-Regular":                 nfBase + "/Tinos/Regular/TinosNerdFont-Regular.ttf",
	"Ubuntu-Regular":                nfBase + "/Ubuntu/Regular/UbuntuNerdFont-Regular.ttf",
	"UbuntuMono-Regular":            nfBase + "/UbuntuMono/Regular/UbuntuMonoNerdFont-Regular.ttf",
	"VictorMono-Regular":            nfBase + "/VictorMono/Regular/VictorMonoNerdFont-Regular.ttf",
	"iAWriterMono-Regular":          nfBase + "/iA-Writer/Mono/Regular/iMWritingMonoNerdFont-Regular.ttf",
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
