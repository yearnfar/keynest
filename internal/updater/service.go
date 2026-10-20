package updater

import (
	"bytes"
	"context"
	"crypto"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/minio/selfupdate"
)

const (
	latestReleaseURL = "https://api.github.com/repos/yearnfar/keynest/releases/latest"
	maxMetadataSize  = 2 << 20
	maxChecksumSize  = 8 << 10
	maxBinarySize    = 250 << 20
	maxErrorBodySize = 64 << 10
	releaseCacheTTL  = 5 * time.Minute
)

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

type githubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	PublishedAt string        `json:"published_at"`
	Draft       bool          `json:"draft"`
	Assets      []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type releaseCandidate struct {
	info          Info
	binaryAsset   githubAsset
	checksumAsset githubAsset
}

type applyFunc func(io.Reader, selfupdate.Options) error

type cachedRelease struct {
	candidate releaseCandidate
	etag      string
	fetchedAt time.Time
}

type fetchResult struct {
	body        []byte
	etag        string
	notModified bool
}

type Service struct {
	apiURL      string
	client      *http.Client
	version     string
	buildMode   string
	goos        string
	goarch      string
	apply       applyFunc
	validateURL func(string) error

	installMu    sync.Mutex
	releaseMu    sync.Mutex
	releaseCache cachedRelease
	restartMu    sync.Mutex
	restartReady bool
}

func NewService() *Service {
	service := &Service{
		apiURL:      latestReleaseURL,
		version:     Version,
		buildMode:   BuildMode,
		goos:        runtime.GOOS,
		goarch:      runtime.GOARCH,
		apply:       selfupdate.Apply,
		validateURL: validateGitHubURL,
	}
	service.client = &http.Client{
		Timeout: 5 * time.Minute,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("更新下载重定向次数过多")
			}
			return service.validateURL(request.URL.String())
		},
	}
	return service
}

func (s *Service) CurrentInfo() Info {
	return Info{
		CurrentVersion: s.version,
		CanAutoUpdate:  s.buildMode == "production",
		StatusMessage:  s.buildStatusMessage(),
	}
}

func (s *Service) Check(ctx context.Context) (Info, error) {
	checkContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	candidate, err := s.latest(checkContext)
	if err != nil {
		return s.CurrentInfo(), err
	}
	return candidate.info, nil
}

func (s *Service) Install(ctx context.Context) (Info, error) {
	s.installMu.Lock()
	defer s.installMu.Unlock()

	if s.buildMode != "production" {
		return s.CurrentInfo(), errors.New("开发版本禁止覆盖自身，请使用正式发布版本测试自动更新")
	}

	checkContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	candidate, err := s.latest(checkContext)
	cancel()
	if err != nil {
		return s.CurrentInfo(), err
	}
	if !candidate.info.Available {
		return candidate.info, errors.New("当前已经是最新版本")
	}
	if !candidate.info.CanAutoUpdate {
		return candidate.info, errors.New("远程版本缺少当前平台的可执行文件或 SHA-256 校验文件")
	}

	checksumBody, err := s.fetch(ctx, candidate.checksumAsset.BrowserDownloadURL, maxChecksumSize)
	if err != nil {
		return candidate.info, fmt.Errorf("下载更新校验文件失败: %w", err)
	}
	checksum, err := parseChecksum(checksumBody)
	if err != nil {
		return candidate.info, fmt.Errorf("更新校验文件无效: %w", err)
	}

	binary, err := s.fetch(ctx, candidate.binaryAsset.BrowserDownloadURL, maxBinarySize)
	if err != nil {
		return candidate.info, fmt.Errorf("下载更新失败: %w", err)
	}
	options := selfupdate.Options{Checksum: checksum, Hash: crypto.SHA256}
	if err := options.CheckPermissions(); err != nil {
		return candidate.info, fmt.Errorf("没有权限更新当前应用: %w", err)
	}
	if err := s.apply(bytes.NewReader(binary), options); err != nil {
		if rollbackErr := selfupdate.RollbackError(err); rollbackErr != nil {
			return candidate.info, fmt.Errorf("更新失败且无法恢复旧版本，请重新下载安装: %v (恢复错误: %v)", err, rollbackErr)
		}
		return candidate.info, fmt.Errorf("安装更新失败: %w", err)
	}

	s.restartMu.Lock()
	s.restartReady = true
	s.restartMu.Unlock()
	candidate.info.StatusMessage = "更新已安装，重启 KeyNest 后生效"
	return candidate.info, nil
}

func (s *Service) Restart() error {
	s.restartMu.Lock()
	ready := s.restartReady
	s.restartMu.Unlock()
	if !ready {
		return errors.New("没有等待重启的更新")
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法定位应用程序: %w", err)
	}
	command := exec.Command(executable, os.Args[1:]...)
	command.Env = os.Environ()
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		return fmt.Errorf("无法启动新版本: %w", err)
	}
	return nil
}

func (s *Service) latest(ctx context.Context) (releaseCandidate, error) {
	s.releaseMu.Lock()
	defer s.releaseMu.Unlock()

	now := time.Now()
	if !s.releaseCache.fetchedAt.IsZero() && now.Sub(s.releaseCache.fetchedAt) < releaseCacheTTL {
		return s.releaseCache.candidate, nil
	}

	result, err := s.fetchConditional(ctx, s.apiURL, maxMetadataSize, s.releaseCache.etag)
	if err != nil {
		return releaseCandidate{}, fmt.Errorf("检查远程版本失败: %w", err)
	}
	if result.notModified {
		if s.releaseCache.fetchedAt.IsZero() {
			return releaseCandidate{}, errors.New("检查远程版本失败: GitHub 返回了无可用缓存的 304 响应")
		}
		s.releaseCache.fetchedAt = now
		return s.releaseCache.candidate, nil
	}
	var release githubRelease
	if err := json.Unmarshal(result.body, &release); err != nil {
		return releaseCandidate{}, fmt.Errorf("远程版本信息无效: %w", err)
	}
	if release.Draft || strings.TrimSpace(release.TagName) == "" {
		return releaseCandidate{}, errors.New("远程版本信息不完整")
	}
	if _, err := parseVersion(release.TagName); err != nil {
		return releaseCandidate{}, fmt.Errorf("远程版本号无效: %w", err)
	}
	comparisonVersion := s.version
	if _, err := parseVersion(comparisonVersion); err != nil {
		if s.buildMode == "production" {
			return releaseCandidate{}, fmt.Errorf("当前版本号无效: %w", err)
		}
		comparisonVersion = "0.0.0"
	}

	assetName := platformAssetName(s.goos, s.goarch)
	var binaryAsset, checksumAsset githubAsset
	for _, asset := range release.Assets {
		switch asset.Name {
		case assetName:
			binaryAsset = asset
		case assetName + ".sha256":
			checksumAsset = asset
		}
	}
	available := compareVersions(release.TagName, comparisonVersion) > 0
	canAutoUpdate := available && s.buildMode == "production" && binaryAsset.Name != "" && checksumAsset.Name != ""
	status := "当前已经是最新版本"
	if available {
		status = "发现新版本 " + displayVersion(release.TagName)
	}
	if available && (binaryAsset.Name == "" || checksumAsset.Name == "") {
		status = "发现新版本，但没有适用于当前平台的自动更新包"
	}
	if available && s.buildMode != "production" {
		status = "发现新版本；开发构建不会覆盖自身"
	}

	candidate := releaseCandidate{
		info: Info{
			CurrentVersion: s.version,
			LatestVersion:  strings.TrimPrefix(release.TagName, "v"),
			ReleaseName:    release.Name,
			ReleaseNotes:   release.Body,
			PublishedAt:    release.PublishedAt,
			ReleaseURL:     release.HTMLURL,
			Available:      available,
			CanAutoUpdate:  canAutoUpdate,
			StatusMessage:  status,
		},
		binaryAsset:   binaryAsset,
		checksumAsset: checksumAsset,
	}
	s.releaseCache = cachedRelease{
		candidate: candidate,
		etag:      result.etag,
		fetchedAt: now,
	}
	return candidate, nil
}

func (s *Service) fetch(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	result, err := s.fetchConditional(ctx, rawURL, limit, "")
	if err != nil {
		return nil, err
	}
	return result.body, nil
}

func (s *Service) fetchConditional(ctx context.Context, rawURL string, limit int64, etag string) (fetchResult, error) {
	if rawURL != s.apiURL {
		if err := s.validateURL(rawURL); err != nil {
			return fetchResult{}, err
		}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fetchResult{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "KeyNest-Updater/"+s.version)
	if etag != "" {
		request.Header.Set("If-None-Match", etag)
	}

	response, err := s.client.Do(request)
	if err != nil {
		return fetchResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotModified && etag != "" {
		return fetchResult{etag: etag, notModified: true}, nil
	}
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBodySize))
		return fetchResult{}, githubResponseError(response.StatusCode, response.Header, body)
	}
	if response.ContentLength > limit {
		return fetchResult{}, fmt.Errorf("下载内容超过 %d 字节限制", limit)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return fetchResult{}, err
	}
	if int64(len(body)) > limit {
		return fetchResult{}, fmt.Errorf("下载内容超过 %d 字节限制", limit)
	}
	return fetchResult{body: body, etag: response.Header.Get("ETag")}, nil
}

func githubResponseError(statusCode int, header http.Header, body []byte) error {
	if statusCode == http.StatusNotFound {
		return errors.New("尚未找到正式发布版本或对应的更新资源")
	}

	message := githubErrorMessage(body)
	if statusCode == http.StatusForbidden || statusCode == http.StatusTooManyRequests {
		if header.Get("X-RateLimit-Remaining") == "0" {
			if reset, err := strconv.ParseInt(header.Get("X-RateLimit-Reset"), 10, 64); err == nil && reset > 0 {
				return fmt.Errorf("GitHub API 请求额度已用尽，请在 %s 后重试", time.Unix(reset, 0).Local().Format("2006-01-02 15:04:05"))
			}
			return errors.New("GitHub API 请求额度已用尽，请稍后重试")
		}
		if retryAfter := retryAfterMessage(header.Get("Retry-After")); retryAfter != "" {
			return fmt.Errorf("GitHub 暂时限制了更新检查频率，请%s后重试", retryAfter)
		}
		lowerMessage := strings.ToLower(message)
		if statusCode == http.StatusTooManyRequests || strings.Contains(lowerMessage, "secondary rate limit") || strings.Contains(lowerMessage, "abuse detection") {
			return errors.New("GitHub 暂时限制了更新检查频率，请至少等待 1 分钟后重试")
		}
		if message != "" {
			return fmt.Errorf("GitHub 拒绝访问：%s", message)
		}
		return errors.New("GitHub 拒绝访问，请稍后重试或确认发布仓库可公开访问")
	}
	return fmt.Errorf("远程服务返回 HTTP %d", statusCode)
}

func githubErrorMessage(body []byte) string {
	var response struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &response) != nil {
		return ""
	}
	message := strings.Join(strings.Fields(response.Message), " ")
	characters := []rune(message)
	if len(characters) > 300 {
		message = string(characters[:300]) + "..."
	}
	return message
}

func retryAfterMessage(value string) string {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds > 0 {
		return fmt.Sprintf("等待 %d 秒", seconds)
	}
	if retryAt, err := http.ParseTime(value); err == nil {
		return fmt.Sprintf("在 %s", retryAt.Local().Format("2006-01-02 15:04:05"))
	}
	return ""
}

func (s *Service) buildStatusMessage() string {
	if s.buildMode == "production" {
		return "可以检查 GitHub Releases 获取更新"
	}
	return "开发构建可以检查版本，但不会覆盖自身"
}

func platformAssetName(goos, goarch string) string {
	name := fmt.Sprintf("keynest-%s-%s", goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

func parseChecksum(value []byte) ([]byte, error) {
	fields := strings.Fields(string(value))
	if len(fields) == 0 || len(fields[0]) != 64 {
		return nil, errors.New("应包含 64 位十六进制 SHA-256")
	}
	checksum, err := hex.DecodeString(fields[0])
	if err != nil {
		return nil, errors.New("SHA-256 不是有效的十六进制字符串")
	}
	return checksum, nil
}

func validateGitHubURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("更新地址无效: %w", err)
	}
	if parsed.Scheme != "https" {
		return errors.New("更新地址必须使用 HTTPS")
	}
	host := strings.ToLower(parsed.Hostname())
	allowed := host == "github.com" || host == "api.github.com" || host == "objects.githubusercontent.com" || host == "release-assets.githubusercontent.com"
	if !allowed {
		return fmt.Errorf("不受信任的更新地址: %s", host)
	}
	return nil
}

type semanticVersion struct {
	major      int64
	minor      int64
	patch      int64
	prerelease []string
}

func parseVersion(value string) (semanticVersion, error) {
	value = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(value, "v"), "V"))
	value = strings.SplitN(value, "+", 2)[0]
	parts := strings.SplitN(value, "-", 2)
	numbers := strings.Split(parts[0], ".")
	if len(numbers) < 1 || len(numbers) > 3 || numbers[0] == "" {
		return semanticVersion{}, fmt.Errorf("%q 不是有效版本号", value)
	}
	parsed := make([]int64, 3)
	for index, number := range numbers {
		if len(number) > 1 && number[0] == '0' {
			return semanticVersion{}, fmt.Errorf("%q 包含前导零", value)
		}
		part, err := strconv.ParseInt(number, 10, 64)
		if err != nil || part < 0 {
			return semanticVersion{}, fmt.Errorf("%q 不是有效版本号", value)
		}
		parsed[index] = part
	}
	version := semanticVersion{major: parsed[0], minor: parsed[1], patch: parsed[2]}
	if len(parts) == 2 {
		if parts[1] == "" {
			return semanticVersion{}, fmt.Errorf("%q 的预发布标识为空", value)
		}
		version.prerelease = strings.Split(parts[1], ".")
	}
	return version, nil
}

func compareVersions(left, right string) int {
	leftVersion, leftErr := parseVersion(left)
	rightVersion, rightErr := parseVersion(right)
	if leftErr != nil || rightErr != nil {
		return 0
	}
	leftNumbers := []int64{leftVersion.major, leftVersion.minor, leftVersion.patch}
	rightNumbers := []int64{rightVersion.major, rightVersion.minor, rightVersion.patch}
	for index := range leftNumbers {
		if leftNumbers[index] < rightNumbers[index] {
			return -1
		}
		if leftNumbers[index] > rightNumbers[index] {
			return 1
		}
	}
	if len(leftVersion.prerelease) == 0 && len(rightVersion.prerelease) > 0 {
		return 1
	}
	if len(leftVersion.prerelease) > 0 && len(rightVersion.prerelease) == 0 {
		return -1
	}
	for index := 0; index < len(leftVersion.prerelease) || index < len(rightVersion.prerelease); index++ {
		if index >= len(leftVersion.prerelease) {
			return -1
		}
		if index >= len(rightVersion.prerelease) {
			return 1
		}
		leftPart, leftNumeric := numericIdentifier(leftVersion.prerelease[index])
		rightPart, rightNumeric := numericIdentifier(rightVersion.prerelease[index])
		switch {
		case leftNumeric && rightNumeric && leftPart < rightPart:
			return -1
		case leftNumeric && rightNumeric && leftPart > rightPart:
			return 1
		case leftNumeric && !rightNumeric:
			return -1
		case !leftNumeric && rightNumeric:
			return 1
		case leftVersion.prerelease[index] < rightVersion.prerelease[index]:
			return -1
		case leftVersion.prerelease[index] > rightVersion.prerelease[index]:
			return 1
		}
	}
	return 0
}

func numericIdentifier(value string) (int64, bool) {
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	return parsed, err == nil
}

func displayVersion(value string) string {
	if strings.HasPrefix(strings.ToLower(value), "v") {
		return value
	}
	return "v" + value
}
