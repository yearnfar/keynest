package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/minio/selfupdate"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		left  string
		right string
		want  int
	}{
		{"v1.2.0", "1.1.9", 1},
		{"1.2", "1.2.0", 0},
		{"1.2.0", "1.2.1", -1},
		{"2.0.0", "1.99.99", 1},
		{"1.0.0", "1.0.0-rc.1", 1},
		{"1.0.0-rc.2", "1.0.0-rc.10", -1},
		{"1.0.0+build.2", "1.0.0+build.1", 0},
	}
	for _, test := range tests {
		t.Run(test.left+"_"+test.right, func(t *testing.T) {
			if got := compareVersions(test.left, test.right); got != test.want {
				t.Fatalf("compareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
			}
		})
	}
}

func TestCheckFindsCompatibleRelease(t *testing.T) {
	service := testService(t, "1.2.3", "production", []byte("new keynest binary"), false)

	info, err := service.Check(context.Background())
	if err != nil {
		t.Fatalf("check update: %v", err)
	}
	if !info.Available || !info.CanAutoUpdate || info.LatestVersion != "1.3.0" {
		t.Fatalf("unexpected update info: %#v", info)
	}
}

func TestCheckReportsCurrentVersion(t *testing.T) {
	service := testService(t, "1.3.0", "production", []byte("new keynest binary"), false)

	info, err := service.Check(context.Background())
	if err != nil {
		t.Fatalf("check update: %v", err)
	}
	if info.Available || info.StatusMessage != "当前已经是最新版本" {
		t.Fatalf("unexpected update info: %#v", info)
	}
}

func TestCheckAcceptsDevelopmentCommitVersion(t *testing.T) {
	service := testService(t, "0db33d09", "development", []byte("new keynest binary"), false)

	info, err := service.Check(context.Background())
	if err != nil {
		t.Fatalf("check update: %v", err)
	}
	if info.CurrentVersion != "0db33d09" || !info.Available || info.CanAutoUpdate {
		t.Fatalf("unexpected development update info: %#v", info)
	}
}

func TestCheckRejectsProductionCommitVersion(t *testing.T) {
	service := testService(t, "0db33d09", "production", []byte("new keynest binary"), false)

	if _, err := service.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "当前版本号无效") {
		t.Fatalf("expected invalid production version error, got %v", err)
	}
}

func TestCheckRequiresChecksumForAutomaticUpdate(t *testing.T) {
	service := testService(t, "1.2.3", "production", []byte("new keynest binary"), true)

	info, err := service.Check(context.Background())
	if err != nil {
		t.Fatalf("check update: %v", err)
	}
	if !info.Available || info.CanAutoUpdate {
		t.Fatalf("automatic update should be disabled without checksum: %#v", info)
	}
}

func TestCheckCachesReleaseMetadataForInstall(t *testing.T) {
	service := testService(t, "1.2.3", "production", []byte("new keynest binary"), false)
	baseTransport := service.client.Transport
	latestRequests := 0
	service.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/latest" {
			latestRequests++
		}
		return baseTransport.RoundTrip(request)
	})
	service.apply = func(io.Reader, selfupdate.Options) error { return nil }

	if _, err := service.Check(context.Background()); err != nil {
		t.Fatalf("check update: %v", err)
	}
	if _, err := service.Install(context.Background()); err != nil {
		t.Fatalf("install update: %v", err)
	}
	if latestRequests != 1 {
		t.Fatalf("latest release requests = %d, want 1", latestRequests)
	}
}

func TestCheckRevalidatesExpiredCacheWithETag(t *testing.T) {
	service := testService(t, "1.2.3", "production", []byte("new keynest binary"), false)
	baseTransport := service.client.Transport
	latestRequests := 0
	service.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/latest" {
			return baseTransport.RoundTrip(request)
		}
		latestRequests++
		if latestRequests == 1 {
			response, err := baseTransport.RoundTrip(request)
			response.Header.Set("ETag", `"release-v1.3.0"`)
			return response, err
		}
		if request.Header.Get("If-None-Match") != `"release-v1.3.0"` {
			t.Fatalf("If-None-Match = %q", request.Header.Get("If-None-Match"))
		}
		return &http.Response{
			StatusCode: http.StatusNotModified,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})

	if _, err := service.Check(context.Background()); err != nil {
		t.Fatalf("initial check: %v", err)
	}
	service.releaseMu.Lock()
	service.releaseCache.fetchedAt = time.Now().Add(-releaseCacheTTL - time.Minute)
	service.releaseMu.Unlock()
	info, err := service.Check(context.Background())
	if err != nil {
		t.Fatalf("revalidated check: %v", err)
	}
	if info.LatestVersion != "1.3.0" || latestRequests != 2 {
		t.Fatalf("unexpected cached result: info=%#v requests=%d", info, latestRequests)
	}
}

func TestCheckExplainsGitHubRateLimits(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		headers    http.Header
		body       string
		want       string
	}{
		{
			name:       "primary limit",
			statusCode: http.StatusForbidden,
			headers: http.Header{
				"X-Ratelimit-Remaining": []string{"0"},
				"X-Ratelimit-Reset":     []string{"1792389974"},
			},
			body: `{"message":"API rate limit exceeded"}`,
			want: "GitHub API 请求额度已用尽",
		},
		{
			name:       "secondary limit with retry after",
			statusCode: http.StatusForbidden,
			headers:    http.Header{"Retry-After": []string{"60"}},
			body:       `{"message":"You have exceeded a secondary rate limit"}`,
			want:       "等待 60 秒",
		},
		{
			name:       "too many requests",
			statusCode: http.StatusTooManyRequests,
			headers:    make(http.Header),
			body:       `{"message":"Too many requests"}`,
			want:       "至少等待 1 分钟",
		},
		{
			name:       "ordinary forbidden",
			statusCode: http.StatusForbidden,
			headers:    http.Header{"X-Ratelimit-Remaining": []string{"45"}},
			body:       `{"message":"Resource not accessible"}`,
			want:       "GitHub 拒绝访问：Resource not accessible",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := errorService(test.statusCode, test.headers, test.body)
			_, err := service.Check(context.Background())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestInstallVerifiesAndAppliesBinary(t *testing.T) {
	binary := []byte("new keynest binary")
	service := testService(t, "1.2.3", "production", binary, false)

	var applied []byte
	service.apply = func(reader io.Reader, options selfupdate.Options) error {
		var err error
		applied, err = io.ReadAll(reader)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(applied)
		if !strings.EqualFold(hex.EncodeToString(digest[:]), hex.EncodeToString(options.Checksum)) {
			return fmt.Errorf("checksum mismatch")
		}
		return nil
	}

	info, err := service.Install(context.Background())
	if err != nil {
		t.Fatalf("install update: %v", err)
	}
	if string(applied) != string(binary) || !strings.Contains(info.StatusMessage, "重启") {
		t.Fatalf("unexpected installation result: info=%#v binary=%q", info, applied)
	}
}

func TestInstallRejectsDevelopmentBuild(t *testing.T) {
	service := testService(t, "1.2.3", "development", []byte("new keynest binary"), false)

	if _, err := service.Install(context.Background()); err == nil || !strings.Contains(err.Error(), "开发版本") {
		t.Fatalf("expected development build rejection, got %v", err)
	}
}

func TestParseChecksum(t *testing.T) {
	digest := strings.Repeat("ab", 32)
	parsed, err := parseChecksum([]byte(digest + "  keynest-darwin-arm64\n"))
	if err != nil {
		t.Fatalf("parse checksum: %v", err)
	}
	if hex.EncodeToString(parsed) != digest {
		t.Fatalf("unexpected checksum: %x", parsed)
	}
	if _, err := parseChecksum([]byte("not-a-checksum")); err == nil {
		t.Fatal("expected invalid checksum error")
	}
}

func testService(t *testing.T, currentVersion, buildMode string, binary []byte, omitChecksum bool) *Service {
	t.Helper()
	digest := sha256.Sum256(binary)
	assets := fmt.Sprintf(`[
        {"name":"keynest-darwin-arm64","browser_download_url":"https://updates.test/binary","size":%d}
    ]`, len(binary))
	if !omitChecksum {
		assets = fmt.Sprintf(`[
            {"name":"keynest-darwin-arm64","browser_download_url":"https://updates.test/binary","size":%d},
            {"name":"keynest-darwin-arm64.sha256","browser_download_url":"https://updates.test/checksum","size":64}
        ]`, len(binary))
	}
	release := fmt.Sprintf(`{"tag_name":"v1.3.0","name":"KeyNest 1.3.0","body":"Changes","html_url":"https://updates.test/release","published_at":"2026-10-19T00:00:00Z","assets":%s}`, assets)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body string
		switch request.URL.Path {
		case "/latest":
			body = release
		case "/binary":
			body = string(binary)
		case "/checksum":
			body = fmt.Sprintf("%x  keynest-darwin-arm64\n", digest)
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("not found")), Header: make(http.Header)}, nil
		}
		return &http.Response{
			StatusCode:    http.StatusOK,
			Body:          io.NopCloser(strings.NewReader(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}, nil
	})}

	service := &Service{
		apiURL:      "https://updates.test/latest",
		client:      client,
		version:     currentVersion,
		buildMode:   buildMode,
		goos:        "darwin",
		goarch:      "arm64",
		apply:       selfupdate.Apply,
		validateURL: func(string) error { return nil },
	}
	return service
}

func errorService(statusCode int, headers http.Header, body string) *Service {
	return &Service{
		apiURL: "https://updates.test/latest",
		client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: statusCode,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     headers,
			}, nil
		})},
		version:     "1.2.3",
		buildMode:   "production",
		goos:        "darwin",
		goarch:      "arm64",
		apply:       selfupdate.Apply,
		validateURL: func(string) error { return nil },
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
