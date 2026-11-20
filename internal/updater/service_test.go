package updater

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	wailsupdater "github.com/wailsapp/wails/v3/pkg/updater"
	githubprovider "github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

func TestCheckMapsVerifiedRelease(t *testing.T) {
	publishedAt := time.Date(2026, time.November, 20, 8, 30, 0, 0, time.UTC)
	engine := &fakeEngine{release: &wailsupdater.Release{
		Version:     "1.3.0",
		Name:        "KeyNest 1.3.0",
		Notes:       "Changes",
		PublishedAt: publishedAt,
		Verification: &wailsupdater.Verification{
			DigestAlgo: "sha256",
			Digest:     make([]byte, 32),
		},
		Metadata: map[string]any{"github.release.htmlURL": "https://github.com/yearnfar/keynest/releases/tag/v1.3.0"},
	}}
	service := newService(engine, "1.2.3", "production")

	info, err := service.Check(context.Background())
	if err != nil {
		t.Fatalf("check update: %v", err)
	}
	if !info.Available || !info.CanAutoUpdate || info.LatestVersion != "1.3.0" {
		t.Fatalf("unexpected update info: %#v", info)
	}
	if info.PublishedAt != publishedAt.Format(time.RFC3339) || info.ReleaseURL == "" {
		t.Fatalf("release metadata was not mapped: %#v", info)
	}
}

func TestCheckReportsCurrentVersion(t *testing.T) {
	service := newService(&fakeEngine{}, "1.3.0", "production")

	info, err := service.Check(context.Background())
	if err != nil {
		t.Fatalf("check update: %v", err)
	}
	if info.Available || info.StatusMessage != "当前已经是最新版本" {
		t.Fatalf("unexpected update info: %#v", info)
	}
}

func TestCheckRequiresMatchingChecksum(t *testing.T) {
	engine := &fakeEngine{release: &wailsupdater.Release{Version: "1.3.0"}}
	service := newService(engine, "1.2.3", "production")

	info, err := service.Check(context.Background())
	if err != nil {
		t.Fatalf("check update: %v", err)
	}
	if !info.Available || info.CanAutoUpdate || !strings.Contains(info.StatusMessage, "SHA-256") {
		t.Fatalf("automatic update should require verification: %#v", info)
	}
}

func TestCheckAllowsDevelopmentVersionWithoutInstalling(t *testing.T) {
	engine := &fakeEngine{release: verifiedRelease()}
	service := newService(engine, "0db33d09", "development")

	info, err := service.Check(context.Background())
	if err != nil {
		t.Fatalf("check update: %v", err)
	}
	if !info.Available || info.CanAutoUpdate || info.CurrentVersion != "0db33d09" {
		t.Fatalf("unexpected development update info: %#v", info)
	}
	if _, err := service.Install(context.Background()); err == nil || !strings.Contains(err.Error(), "开发版本") {
		t.Fatalf("expected development install rejection, got %v", err)
	}
}

func TestCheckRejectsInvalidProductionVersion(t *testing.T) {
	service := newService(&fakeEngine{release: verifiedRelease()}, "0db33d09", "production")
	if _, err := service.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "当前版本号无效") {
		t.Fatalf("expected invalid production version error, got %v", err)
	}
}

func TestInstallAndRestartDelegateToWailsUpdater(t *testing.T) {
	engine := &fakeEngine{release: verifiedRelease()}
	service := newService(engine, "1.2.3", "production")
	if _, err := service.Check(context.Background()); err != nil {
		t.Fatalf("check update: %v", err)
	}
	info, err := service.Install(context.Background())
	if err != nil {
		t.Fatalf("install update: %v", err)
	}
	if engine.downloadCalls != 1 || !strings.Contains(info.StatusMessage, "重启") {
		t.Fatalf("unexpected install result: calls=%d info=%#v", engine.downloadCalls, info)
	}
	if err := service.Restart(context.Background()); err != nil {
		t.Fatalf("restart update: %v", err)
	}
	if engine.restartCalls != 1 {
		t.Fatalf("restart calls = %d, want 1", engine.restartCalls)
	}
}

func TestPlatformAssetNamesAndExactMatcher(t *testing.T) {
	tests := []struct {
		goos, goarch, want string
	}{
		{"darwin", "arm64", "keynest-darwin-arm64.zip"},
		{"windows", "amd64", "keynest-windows-amd64.exe"},
		{"linux", "amd64", "keynest-linux-amd64"},
	}
	for _, test := range tests {
		if got := platformAssetName(test.goos, test.goarch); got != test.want {
			t.Errorf("platformAssetName(%q, %q) = %q, want %q", test.goos, test.goarch, got, test.want)
		}
	}

	assets := []githubprovider.ReleaseAsset{
		{Name: "KeyNest-macOS-arm64.dmg"},
		{Name: "keynest-darwin-arm64.zip.sha256"},
		{Name: "keynest-darwin-arm64.zip"},
	}
	matcher := exactAssetMatcher("keynest-darwin-arm64.zip")
	if got := matcher(wailsupdater.CheckRequest{}, assets); got != 2 {
		t.Fatalf("exact matcher selected index %d, want 2", got)
	}
}

func TestValidateGitHubURL(t *testing.T) {
	allowed := []string{
		"https://api.github.com/repos/yearnfar/keynest/releases/latest",
		"https://release-assets.githubusercontent.com/file",
		"https://objects.githubusercontent.com/file",
	}
	for _, rawURL := range allowed {
		parsed, _ := url.Parse(rawURL)
		if err := validateGitHubURL(parsed); err != nil {
			t.Errorf("validateGitHubURL(%q): %v", rawURL, err)
		}
	}

	rejected := []string{"http://api.github.com/file", "https://github.example.com/file", "https://evil.test/file"}
	for _, rawURL := range rejected {
		parsed, _ := url.Parse(rawURL)
		if err := validateGitHubURL(parsed); err == nil {
			t.Errorf("validateGitHubURL(%q) unexpectedly succeeded", rawURL)
		}
	}
}

func TestTrustedTransportRejectsRequestBeforeNetwork(t *testing.T) {
	baseCalled := false
	transport := trustedGitHubTransport{base: roundTripFunc(func(*http.Request) (*http.Response, error) {
		baseCalled = true
		return nil, errors.New("unexpected network call")
	})}
	request, _ := http.NewRequest(http.MethodGet, "https://evil.test/update", nil)
	if _, err := transport.RoundTrip(request); err == nil || baseCalled {
		t.Fatalf("transport error=%v baseCalled=%v", err, baseCalled)
	}
}

func verifiedRelease() *wailsupdater.Release {
	return &wailsupdater.Release{
		Version: "1.3.0",
		Verification: &wailsupdater.Verification{
			DigestAlgo: "sha256",
			Digest:     make([]byte, 32),
		},
	}
}

type fakeEngine struct {
	release       *wailsupdater.Release
	checkErr      error
	downloadErr   error
	restartErr    error
	downloadCalls int
	restartCalls  int
}

func (engine *fakeEngine) Check(context.Context) (*wailsupdater.Release, error) {
	return engine.release, engine.checkErr
}

func (engine *fakeEngine) DownloadAndInstall(context.Context) error {
	engine.downloadCalls++
	return engine.downloadErr
}

func (engine *fakeEngine) Restart(context.Context) error {
	engine.restartCalls++
	return engine.restartErr
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
