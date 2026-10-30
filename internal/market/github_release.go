package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ReleaseSource describes one GitHub release asset candidate for auto-sync.
type ReleaseSource struct {
	Owner, Repo string
	Tag         string
	ReleaseURL  string
	AssetURL    string
	AssetName   string
	Prerelease  bool
}

var githubRepoPattern = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9-]+)/([A-Za-z0-9_.-]+)/?$`)
var githubReleasePattern = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9-]+)/([A-Za-z0-9_.-]+)/releases/tag/([A-Za-z0-9._+/-]+?)/?$`)
var githubReleaseDownloadPattern = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9-]+)/([A-Za-z0-9_.-]+)/releases/download/([A-Za-z0-9._+/-]+?)/([A-Za-z0-9._+ -]+?)$`)

// parseReleaseReference accepts either a repository root or an exact Release
// page/download. A bare owner/repo URL is the single-plugin convenience path;
// a monorepo must point at one tag so the plugin identity is unambiguous.
func parseReleaseReference(raw string) (RepositoryRef, error) {
	value := strings.TrimSpace(raw)
	if match := githubRepoPattern.FindStringSubmatch(value); match != nil {
		return RepositoryRef{Owner: match[1], Repo: match[2]}, nil
	}
	if match := githubReleasePattern.FindStringSubmatch(value); match != nil {
		return RepositoryRef{Owner: match[1], Repo: match[2], Tag: match[3]}, nil
	}
	if match := githubReleaseDownloadPattern.FindStringSubmatch(value); match != nil {
		return RepositoryRef{Owner: match[1], Repo: match[2], Tag: match[3], AssetName: match[4]}, nil
	}
	return RepositoryRef{}, ErrInvalid
}

// RepositoryRef describes a repository or one release in it.
type RepositoryRef struct {
	Owner, Repo string
	Tag         string
	AssetName   string
}

func (r RepositoryRef) RepositoryURL() string {
	return "https://github.com/" + r.Owner + "/" + r.Repo
}

// parseRepositoryURL extracts owner/repo from a canonical GitHub repository URL.
func parseRepositoryURL(raw string) (string, string, error) {
	match := githubRepoPattern.FindStringSubmatch(strings.TrimSpace(raw))
	if match == nil {
		return "", "", ErrInvalid
	}
	return match[1], match[2], nil
}

// ReleaseFetcher reads release metadata and asset bytes so tests can stub GitHub.
type ReleaseFetcher interface {
	LatestRelease(ctx context.Context, owner, repo string) (ReleaseSource, error)
	FetchAsset(ctx context.Context, url string) ([]byte, error)
}

// TaggedReleaseFetcher resolves the newest release belonging to one plugin's tag
// stream. Official plugins share a single repository, so the repository-wide
// "latest release" belongs to whichever plugin was tagged last; a prefix such as
// "mongo-shell-v" pins the sync to the right stream.
type TaggedReleaseFetcher interface {
	LatestTaggedRelease(ctx context.Context, owner, repo, tagPrefix string) (ReleaseSource, error)
}

// ExactReleaseFetcher is implemented by the production fetcher. Keeping it
// separate preserves the existing release-sync test seam.
type ExactReleaseFetcher interface {
	Release(context.Context, RepositoryRef) (ReleaseSource, error)
	FetchAsset(context.Context, string) ([]byte, error)
}

// GitHubReleaseFetcher talks to the public GitHub API; an optional token lifts
// the anonymous 60 requests/hour ceiling for larger catalogs.
type GitHubReleaseFetcher struct {
	client  *http.Client
	token   string
	baseURL string
}

// NewGitHubReleaseFetcher creates the production fetcher with bounded timeouts.
func NewGitHubReleaseFetcher(token string) *GitHubReleaseFetcher {
	return &GitHubReleaseFetcher{client: &http.Client{Timeout: 2 * time.Minute}, token: token, baseURL: "https://api.github.com"}
}

type githubRelease struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Release resolves one exact tag or the repository's latest stable release.
// An explicit tag permits prereleases; the implicit latest lookup does not.
func (f *GitHubReleaseFetcher) Release(ctx context.Context, ref RepositoryRef) (ReleaseSource, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/releases/latest", f.apiBase(), ref.Owner, ref.Repo)
	if ref.Tag != "" {
		endpoint = fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", f.apiBase(), ref.Owner, ref.Repo, url.PathEscape(ref.Tag))
	}
	return f.release(ctx, ref, endpoint, ref.Tag != "")
}

// LatestRelease returns the single archive asset of the repository's latest
// non-draft, non-prerelease release. Ambiguous or asset-less releases are errors,
// never a guess: a wrong archive would publish under the wrong plugin identity.
func (f *GitHubReleaseFetcher) LatestRelease(ctx context.Context, owner, repo string) (ReleaseSource, error) {
	return f.Release(ctx, RepositoryRef{Owner: owner, Repo: repo})
}

// LatestTaggedRelease walks the release list and keeps the highest version whose
// tag carries the plugin prefix; drafts and prereleases are never candidates.
func (f *GitHubReleaseFetcher) LatestTaggedRelease(ctx context.Context, owner, repo, tagPrefix string) (ReleaseSource, error) {
	if tagPrefix == "" {
		return f.LatestRelease(ctx, owner, repo)
	}
	best := ""
	for page := 1; page <= 10; page++ {
		releases, err := f.listReleases(ctx, owner, repo, page)
		if err != nil {
			return ReleaseSource{}, err
		}
		for _, release := range releases {
			if release.Draft || release.Prerelease || !strings.HasPrefix(release.TagName, tagPrefix) {
				continue
			}
			version := strings.TrimPrefix(release.TagName, tagPrefix)
			if version == "" {
				continue
			}
			if best == "" || versionNewer(version, best) {
				best = version
			}
		}
		if len(releases) < releaseListPageSize {
			break
		}
	}
	if best == "" {
		return ReleaseSource{}, ErrNotFound
	}
	return f.Release(ctx, RepositoryRef{Owner: owner, Repo: repo, Tag: tagPrefix + best})
}

const releaseListPageSize = 100

func (f *GitHubReleaseFetcher) apiBase() string {
	if f.baseURL != "" {
		return f.baseURL
	}
	return "https://api.github.com"
}

func (f *GitHubReleaseFetcher) listReleases(ctx context.Context, owner, repo string, page int) ([]githubRelease, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=%d&page=%d", f.apiBase(), owner, repo, releaseListPageSize, page)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, ErrInvalid
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	if f.token != "" {
		request.Header.Set("Authorization", "Bearer "+f.token)
	}
	response, err := f.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("release list request failed")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release list status %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("release list read failed")
	}
	var releases []githubRelease
	if err = json.Unmarshal(data, &releases); err != nil {
		return nil, ErrInvalid
	}
	return releases, nil
}

func (f *GitHubReleaseFetcher) release(ctx context.Context, ref RepositoryRef, endpoint string, explicitTag bool) (ReleaseSource, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ReleaseSource{}, ErrInvalid
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	if f.token != "" {
		request.Header.Set("Authorization", "Bearer "+f.token)
	}
	response, err := f.client.Do(request)
	if err != nil {
		return ReleaseSource{}, fmt.Errorf("release request failed")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return ReleaseSource{}, ErrNotFound
	}
	if response.StatusCode != http.StatusOK {
		return ReleaseSource{}, fmt.Errorf("release request status %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return ReleaseSource{}, fmt.Errorf("release metadata read failed")
	}
	var release githubRelease
	if err = json.Unmarshal(data, &release); err != nil || release.TagName == "" {
		return ReleaseSource{}, ErrInvalid
	}
	if release.Draft || (!explicitTag && release.Prerelease) {
		return ReleaseSource{}, ErrNotFound
	}
	if ref.AssetName != "" {
		for _, asset := range release.Assets {
			if asset.Name == ref.AssetName && asset.BrowserDownloadURL != "" {
				return ReleaseSource{Owner: ref.Owner, Repo: ref.Repo, Tag: release.TagName, ReleaseURL: release.HTMLURL,
					AssetURL: asset.BrowserDownloadURL, AssetName: asset.Name, Prerelease: release.Prerelease}, nil
			}
		}
		return ReleaseSource{}, ErrNotFound
	}
	var candidates []int
	for index, asset := range release.Assets {
		lower := strings.ToLower(asset.Name)
		if (ref.AssetName == "" || asset.Name == ref.AssetName) && (strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".tar.gz")) {
			candidates = append(candidates, index)
		}
	}
	if len(candidates) != 1 {
		return ReleaseSource{}, ErrConflict
	}
	asset := release.Assets[candidates[0]]
	if asset.BrowserDownloadURL == "" {
		return ReleaseSource{}, ErrInvalid
	}
	releaseURL := strings.TrimSpace(release.HTMLURL)
	if releaseURL == "" {
		releaseURL = "https://github.com/" + ref.Owner + "/" + ref.Repo + "/releases/tag/" + release.TagName
	}
	return ReleaseSource{Owner: ref.Owner, Repo: ref.Repo, Tag: release.TagName, ReleaseURL: releaseURL,
		AssetURL: asset.BrowserDownloadURL, AssetName: asset.Name, Prerelease: release.Prerelease}, nil
}

// FetchAsset downloads one release archive bounded by the package size limit.
func (f *GitHubReleaseFetcher) FetchAsset(ctx context.Context, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, ErrInvalid
	}
	if f.token != "" {
		request.Header.Set("Authorization", "Bearer "+f.token)
	}
	response, err := f.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("asset download failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("asset download status %d", response.StatusCode)
	}
	return io.ReadAll(io.LimitReader(response.Body, MaxArtifactBytes+1))
}
