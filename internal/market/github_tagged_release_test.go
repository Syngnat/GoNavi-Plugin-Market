package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A shared repository publishes one release stream per plugin; the sync must pick
// this plugin's highest version instead of the repository's newest tag.
func TestLatestTaggedReleaseIgnoresOtherPluginStreams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/plugins/releases":
			if r.URL.Query().Get("page") != "1" {
				_, _ = w.Write([]byte("[]"))
				return
			}
			_, _ = w.Write([]byte(`[
				{"tag_name":"other-plugin-v9.9.9","draft":false,"prerelease":false},
				{"tag_name":"mongo-shell-v1.0.3","draft":false,"prerelease":false},
				{"tag_name":"mongo-shell-v1.0.4","draft":false,"prerelease":false},
				{"tag_name":"mongo-shell-v9.0.0","draft":true,"prerelease":false}
			]`))
		case "/repos/acme/plugins/releases/tags/mongo-shell-v1.0.4":
			_, _ = w.Write([]byte(`{"tag_name":"mongo-shell-v1.0.4","html_url":"https://github.com/acme/plugins/releases/tag/mongo-shell-v1.0.4","assets":[{"name":"mongo-shell.zip","browser_download_url":"https://example.test/mongo-shell.zip"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	fetcher := NewGitHubReleaseFetcher("")
	fetcher.baseURL = server.URL
	release, err := fetcher.LatestTaggedRelease(context.Background(), "acme", "plugins", "mongo-shell-v")
	if err != nil {
		t.Fatalf("LatestTaggedRelease returned error: %v", err)
	}
	if release.Tag != "mongo-shell-v1.0.4" || release.AssetName != "mongo-shell.zip" {
		t.Fatalf("picked wrong release: %+v", release)
	}
}

// Without a matching stream the sync must report absence instead of falling back
// to the repository's latest release.
func TestLatestTaggedReleaseRequiresMatchingPrefix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/acme/plugins/releases" {
			_, _ = w.Write([]byte(`[{"tag_name":"other-plugin-v9.9.9","draft":false,"prerelease":false}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	fetcher := NewGitHubReleaseFetcher("")
	fetcher.baseURL = server.URL
	if _, err := fetcher.LatestTaggedRelease(context.Background(), "acme", "plugins", "mongo-shell-v"); err == nil {
		t.Fatal("expected ErrNotFound when the prefix matches no release")
	}
}

// A fetcher without tag support must make the pass skip the plugin rather than
// publish whatever the repository's latest tag happens to be.
func TestPluginReleaseRejectsFetcherWithoutTagSupport(t *testing.T) {
	server := &Server{releases: &stubReleases{}}
	_, err := server.pluginRelease(context.Background(), "acme", "plugins", SourcePlugin{PluginID: "x", TagPrefix: "mongo-shell-v"})
	if err == nil || err != ErrTagPrefixUnsupported {
		t.Fatalf("expected ErrTagPrefixUnsupported, got %v", err)
	}
}
