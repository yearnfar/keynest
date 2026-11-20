package updater

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsupdater "github.com/wailsapp/wails/v3/pkg/updater"
	githubprovider "github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

const repository = "yearnfar/keynest"

// Version and BuildMode are overridden by build commands through -ldflags.
var (
	Version   = "0.1.0"
	BuildMode = "development"
)

type Info struct {
	CurrentVersion string
	LatestVersion  string
	ReleaseName    string
	ReleaseNotes   string
	PublishedAt    string
	ReleaseURL     string
	Available      bool
	CanAutoUpdate  bool
	StatusMessage  string
}

type updateEngine interface {
	Check(context.Context) (*wailsupdater.Release, error)
	DownloadAndInstall(context.Context) error
	Restart(context.Context) error
}

type Service struct {
	engine    updateEngine
	version   string
	buildMode string

	mu   sync.RWMutex
	info Info
}

func NewService(engine *wailsupdater.Updater) (*Service, error) {
	assetName := platformAssetName(runtime.GOOS, runtime.GOARCH)
	provider, err := githubprovider.New(githubprovider.Config{
		Repository:    repository,
		HTTPClient:    newGitHubClient(),
		AssetMatcher:  exactAssetMatcher(assetName),
		ChecksumAsset: assetName + ".sha256",
	})
	if err != nil {
		return nil, fmt.Errorf("配置 GitHub 更新源失败: %w", err)
	}
	if err := engine.Init(wailsupdater.Config{
		CurrentVersion: Version,
		Providers:      []wailsupdater.Provider{provider},
		Window:         wailsupdater.WindowNone,
	}); err != nil {
		return nil, fmt.Errorf("初始化 Wails 更新器失败: %w", err)
	}
	return newService(engine, Version, BuildMode), nil
}

func newService(engine updateEngine, version, buildMode string) *Service {
	service := &Service{engine: engine, version: version, buildMode: buildMode}
	service.info = service.baseInfo()
	return service
}

func (s *Service) CurrentInfo() Info {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.info
}

func (s *Service) Check(ctx context.Context) (Info, error) {
	if s.buildMode == "production" && !isReleaseVersion(s.version) {
		return s.CurrentInfo(), fmt.Errorf("当前版本号无效: %q", s.version)
	}

	release, err := s.engine.Check(ctx)
	if err != nil {
		return s.CurrentInfo(), fmt.Errorf("检查远程版本失败: %w", err)
	}
	if release == nil {
		info := s.baseInfo()
		info.StatusMessage = "当前已经是最新版本"
		s.storeInfo(info)
		return info, nil
	}

	verified := release.Verification != nil &&
		strings.EqualFold(release.Verification.DigestAlgo, "sha256") &&
		len(release.Verification.Digest) == 32
	info := Info{
		CurrentVersion: s.version,
		LatestVersion:  release.Version,
		ReleaseName:    release.Name,
		ReleaseNotes:   release.Notes,
		PublishedAt:    formatPublishedAt(release.PublishedAt),
		ReleaseURL:     releaseMetadataString(release, "github.release.htmlURL"),
		Available:      true,
		CanAutoUpdate:  s.buildMode == "production" && verified,
		StatusMessage:  "发现新版本 " + displayVersion(release.Version),
	}
	if !verified {
		info.StatusMessage = "发现新版本，但缺少匹配的 SHA-256 校验文件"
	} else if s.buildMode != "production" {
		info.StatusMessage = "发现新版本；开发构建不会覆盖自身"
	}
	s.storeInfo(info)
	return info, nil
}

func (s *Service) Install(ctx context.Context) (Info, error) {
	if s.buildMode != "production" {
		return s.CurrentInfo(), errors.New("开发版本禁止覆盖自身，请使用正式发布版本测试自动更新")
	}
	info := s.CurrentInfo()
	if !info.Available {
		return info, errors.New("请先检查更新")
	}
	if !info.CanAutoUpdate {
		return info, errors.New("远程版本缺少当前平台的更新包或匹配的 SHA-256 校验文件")
	}
	if err := s.engine.DownloadAndInstall(ctx); err != nil {
		return info, fmt.Errorf("安装更新失败: %w", err)
	}
	info.StatusMessage = "更新已准备完成，重启 KeyNest 后生效"
	s.storeInfo(info)
	return info, nil
}

func (s *Service) Restart(ctx context.Context) error {
	if err := s.engine.Restart(ctx); err != nil {
		return fmt.Errorf("重启并应用更新失败: %w", err)
	}
	return nil
}

func (s *Service) baseInfo() Info {
	status := "可以检查 GitHub Releases 获取更新"
	if s.buildMode != "production" {
		status = "开发构建可以检查版本，但不会覆盖自身"
	}
	return Info{CurrentVersion: s.version, StatusMessage: status}
}

func (s *Service) storeInfo(info Info) {
	s.mu.Lock()
	s.info = info
	s.mu.Unlock()
}

func platformAssetName(goos, goarch string) string {
	name := fmt.Sprintf("keynest-%s-%s", goos, goarch)
	switch goos {
	case "darwin":
		name += ".zip"
	case "windows":
		name += ".exe"
	}
	return name
}

func exactAssetMatcher(name string) githubprovider.AssetMatcher {
	return func(_ wailsupdater.CheckRequest, assets []githubprovider.ReleaseAsset) int {
		for index, asset := range assets {
			if strings.EqualFold(asset.Name, name) {
				return index
			}
		}
		return -1
	}
}

func newGitHubClient() *http.Client {
	return &http.Client{
		Timeout:   5 * time.Minute,
		Transport: trustedGitHubTransport{base: http.DefaultTransport},
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("更新下载重定向次数过多")
			}
			return validateGitHubURL(request.URL)
		},
	}
}

type trustedGitHubTransport struct {
	base http.RoundTripper
}

func (transport trustedGitHubTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if err := validateGitHubURL(request.URL); err != nil {
		return nil, err
	}
	return transport.base.RoundTrip(request)
}

func validateGitHubURL(value *url.URL) error {
	if value == nil {
		return errors.New("更新地址无效")
	}
	if value.Scheme != "https" {
		return errors.New("更新地址必须使用 HTTPS")
	}
	host := strings.ToLower(value.Hostname())
	allowed := host == "github.com" || host == "api.github.com" ||
		host == "objects.githubusercontent.com" || host == "release-assets.githubusercontent.com"
	if !allowed {
		return fmt.Errorf("不受信任的更新地址: %s", host)
	}
	return nil
}

func releaseMetadataString(release *wailsupdater.Release, key string) string {
	if release == nil || release.Metadata == nil {
		return ""
	}
	value, _ := release.Metadata[key].(string)
	return value
}

func formatPublishedAt(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func isReleaseVersion(value string) bool {
	value = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(value), "v"), "V")
	core := strings.SplitN(strings.SplitN(value, "+", 2)[0], "-", 2)[0]
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return false
		}
		for _, character := range part {
			if character < '0' || character > '9' {
				return false
			}
		}
	}
	return true
}

func displayVersion(value string) string {
	if strings.HasPrefix(strings.ToLower(value), "v") {
		return value
	}
	return "v" + value
}
