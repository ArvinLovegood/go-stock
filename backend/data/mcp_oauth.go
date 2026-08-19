package data

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"

	"gorm.io/gorm"
)

// MCP OAuth 2.1 鉴权模块（按 MCP 官方 Authorization 规范 + RFC 8414/7591/7636 实现）。
//
// 已验证的目标端点（腾讯自选股 MCP）完整链路：
//  1. 401 响应头 WWW-Authenticate: Bearer resource_metadata=<PR元数据URL>（RFC 9727）
//  2. PR 元数据 → authorization_servers
//  3. AS 元数据（RFC 8414）→ authorize/token/register 端点、PKCE S256、public client
//  4. 动态客户端注册（RFC 7591，token_endpoint_auth_method=none 无需 secret）
//  5. PKCE S256 + loopback 回调（http://localhost:port/callback）收 authorization code
//  6. token 交换拿 access_token/refresh_token，Bearer 头访问 MCP
//  7. 过期前用 refresh_token 自动刷新
//
// 凭证存储：AES-256-GCM 加密后落 MCPServer.AuthConfig。密钥由机器特征派生，
// 跨机器迁移库文件时解密失败会优雅降级（清凭证要求重新授权），不阻塞启动。

// oauthProtectedResource RFC 9727 保护资源元数据
type oauthProtectedResource struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
}

// oauthAuthorizationServer RFC 8414 授权服务器元数据
type oauthAuthorizationServer struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint"`
	ScopesSupported                   []string `json:"scopes_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
}

// oauthRegistrationResponse RFC 7591 动态注册响应
type oauthRegistrationResponse struct {
	ClientID                string   `json:"client_id"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

// oauthTokenResponse RFC 6749 token 响应
type oauthTokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"`
	Scope            string `json:"scope"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// MCPAuthConfig 落库凭证（加密前结构）
type MCPAuthConfig struct {
	ClientID         string `json:"clientId"`
	RedirectURI      string `json:"redirectUri"`
	AuthorizationURL string `json:"authorizationUrl"`
	TokenURL         string `json:"tokenUrl"`
	RegisterURL      string `json:"registerUrl"`
	Scopes           string `json:"scopes"`
	AccessToken      string `json:"accessToken"`
	RefreshToken     string `json:"refreshToken"`
	TokenType        string `json:"tokenType"`
	ExpiresAt        int64  `json:"expiresAt"` // Unix 秒
}

// oauthPendingFlow 进行中的授权流（loopback 回调前保存）
type oauthPendingFlow struct {
	State       string
	Verifier    string
	ClientID    string
	RedirectURI string
	TokenURL    string
	Server      *http.Server
	CreatedAt   time.Time
}

var (
	oauthFlowMu     sync.Mutex
	oauthFlows      = make(map[uint]*oauthPendingFlow)
	oauthHTTPClient = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
)

// loopback 回调端口候选：固定区间优先（与注册的 redirect_uri 匹配），冲突时顺延
var oauthCallbackPorts = []int{18963, 18964, 18965, 18966, 18967, 18968}

// ------------------------- 凭证加解密（AES-256-GCM） -------------------------

// mcpAuthKey 机器特征派生密钥。同机稳定，跨机变化（迁移库文件时解密失败 → 重新授权）。
func mcpAuthKey() []byte {
	host, _ := os.Hostname()
	home, _ := os.UserHomeDir()
	exe := os.Args[0]
	sum := sha256.Sum256([]byte("go-stock|mcp-oauth|" + host + "|" + home + "|" + exe))
	return sum[:] // 32 字节 = AES-256
}

func encryptAuthConfig(cfg *MCPAuthConfig) (string, error) {
	plain, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(mcpAuthKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, plain, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func decryptAuthConfig(sealed string) (*MCPAuthConfig, error) {
	if sealed == "" {
		return nil, fmt.Errorf("无凭证")
	}
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(mcpAuthKey())
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("密文损坏")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("凭证解密失败（可能由其他机器迁移而来）")
	}
	var cfg MCPAuthConfig
	if err := json.Unmarshal(plain, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveAuthConfig 加密凭证并落库。走 gorm Updates 自动刷新 UpdatedAt，
// agent 层工具缓存指纹含 UpdatedAt.UnixNano()，凭证轮换后自动失效重建。
func (a *MCPServerApi) SaveAuthConfig(id uint, cfg *MCPAuthConfig) error {
	sealed, err := encryptAuthConfig(cfg)
	if err != nil {
		return err
	}
	expire := time.Time{}
	if cfg.ExpiresAt > 0 {
		expire = time.Unix(cfg.ExpiresAt, 0)
	}
	return dbDao().Model(&models.MCPServer{}).Where("id = ?", id).Updates(map[string]any{
		"auth_config":     sealed,
		"token_expire_at": expire,
	}).Error
}

// LoadAuthConfig 读取并解密凭证。解密失败返回错误（调用方降级）。
func (a *MCPServerApi) LoadAuthConfig(id uint) (*MCPAuthConfig, error) {
	server, err := a.GetByID(id)
	if err != nil {
		return nil, err
	}
	return decryptAuthConfig(server.AuthConfig)
}

// ------------------------- 元数据发现（RFC 9727 → RFC 8414） -------------------------

func wellKnownURL(base string) string {
	// https://host[:port]/path → https://host[:port]/.well-known/oauth-protected-resource
	u, err := url.Parse(base)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host + "/.well-known/oauth-protected-resource"
}

// DiscoverOAuthMetadata 从 MCP 端点 URL 出发发现 AS 元数据。
// base 为空时默认腾讯自选股（go-stock 主要接入目标）。
func DiscoverOAuthMetadata(ctx context.Context, base string) (*oauthAuthorizationServer, error) {
	if base == "" {
		base = "https://stockbuddy.qq.com/cgi/cgi-bin/openai/mcp/mcp"
	}
	prURL := wellKnownURL(base)
	if prURL == "" {
		return nil, fmt.Errorf("无效的 MCP URL: %s", base)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, prURL, nil)
	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取保护资源元数据失败: %w", err)
	}
	defer resp.Body.Close()
	var pr oauthProtectedResource
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("解析保护资源元数据失败: %w", err)
	}
	if len(pr.AuthorizationServers) == 0 {
		return nil, fmt.Errorf("元数据未包含 authorization_servers")
	}

	asURL := strings.TrimRight(pr.AuthorizationServers[0], "/") + "/.well-known/oauth-authorization-server"
	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, asURL, nil)
	resp2, err := oauthHTTPClient.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("获取授权服务器元数据失败: %w", err)
	}
	defer resp2.Body.Close()
	var as oauthAuthorizationServer
	if err := json.NewDecoder(resp2.Body).Decode(&as); err != nil {
		return nil, fmt.Errorf("解析授权服务器元数据失败: %w", err)
	}
	if as.AuthorizationEndpoint == "" || as.TokenEndpoint == "" {
		return nil, fmt.Errorf("元数据缺少 authorize/token 端点")
	}
	return &as, nil
}

// registerOAuthClient RFC 7591 动态客户端注册（public client，无需 secret）
func registerOAuthClient(ctx context.Context, as *oauthAuthorizationServer, redirectURI string) (*oauthRegistrationResponse, error) {
	if as.RegistrationEndpoint == "" {
		return nil, fmt.Errorf("授权服务器不支持动态客户端注册")
	}
	body := map[string]any{
		"client_name":                "go-stock",
		"redirect_uris":              []string{redirectURI},
		"token_endpoint_auth_method": "none",
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
	}
	bs, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, as.RegistrationEndpoint, strings.NewReader(string(bs)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("动态注册请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("动态注册失败 HTTP %d: %s", resp.StatusCode, string(b))
	}
	var reg oauthRegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil {
		return nil, err
	}
	if reg.ClientID == "" {
		return nil, fmt.Errorf("注册响应缺少 client_id")
	}
	return &reg, nil
}

// ------------------------- PKCE（RFC 7636，S256） -------------------------

func generatePKCE() (verifier, challenge string, err error) {
	buf := make([]byte, 48)
	if _, err = rand.Read(buf); err != nil {
		return
	}
	verifier = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return
}

func randomState() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// ------------------------- 授权流（loopback 回调） -------------------------

// StartOAuth 启动 OAuth 授权：发现元数据 → （复用或新建）客户端注册 →
// 起 loopback HTTP 服务等回调 → 返回授权 URL（调用方拉起系统浏览器）。
func (a *MCPServerApi) StartOAuth(ctx context.Context, id uint) (string, error) {
	server, err := a.GetByID(id)
	if err != nil {
		return "", err
	}

	// 清理同服务器旧流程
	oauthFlowMu.Lock()
	if old, ok := oauthFlows[id]; ok && old.Server != nil {
		_ = old.Server.Close()
	}
	delete(oauthFlows, id)
	oauthFlowMu.Unlock()

	as, err := DiscoverOAuthMetadata(ctx, server.URL)
	if err != nil {
		return "", err
	}

	// 找可用回调端口
	var ln net.Listener
	var redirectURI string
	for _, port := range oauthCallbackPorts {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			ln = l
			redirectURI = fmt.Sprintf("http://localhost:%d/callback", port)
			break
		}
	}
	if ln == nil {
		return "", fmt.Errorf("回调端口 %d-%d 均被占用", oauthCallbackPorts[0], oauthCallbackPorts[len(oauthCallbackPorts)-1])
	}

	// 复用已注册的 client_id（redirect_uri 必须与注册时一致），否则重新注册
	var clientID string
	if cfg, cerr := decryptAuthConfig(server.AuthConfig); cerr == nil &&
		cfg.ClientID != "" && cfg.RedirectURI == redirectURI {
		clientID = cfg.ClientID
	} else {
		reg, rerr := registerOAuthClient(ctx, as, redirectURI)
		if rerr != nil {
			_ = ln.Close()
			return "", rerr
		}
		clientID = reg.ClientID
	}

	verifier, challenge, err := generatePKCE()
	if err != nil {
		_ = ln.Close()
		return "", err
	}
	state, err := randomState()
	if err != nil {
		_ = ln.Close()
		return "", err
	}

	scope := strings.Join(as.ScopesSupported, " ")

	// 构造授权 URL
	au, err := url.Parse(as.AuthorizationEndpoint)
	if err != nil {
		_ = ln.Close()
		return "", err
	}
	q := au.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	if scope != "" {
		q.Set("scope", scope)
	}
	q.Set("state", state)
	au.RawQuery = q.Encode()

	flow := &oauthPendingFlow{
		State:       state,
		Verifier:    verifier,
		ClientID:    clientID,
		RedirectURI: redirectURI,
		TokenURL:    as.TokenEndpoint,
		CreatedAt:   time.Now(),
	}

	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux}
	flow.Server = srv

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		a.handleOAuthCallback(w, r, id, flow)
	})

	// 回调服务：授权完成或 3 分钟超时自动关闭
	go func() {
		_ = srv.Serve(ln)
	}()
	go func() {
		time.Sleep(3 * time.Minute)
		oauthFlowMu.Lock()
		if f, ok := oauthFlows[id]; ok && f == flow {
			_ = srv.Close()
			delete(oauthFlows, id)
			a.UpdateStatus(id, "unauthorized", "授权超时未完成，请重试")
		}
		oauthFlowMu.Unlock()
	}()

	oauthFlowMu.Lock()
	oauthFlows[id] = flow
	oauthFlowMu.Unlock()

	// 先保存 client_id/端点信息（token 字段为空），注册信息不因超时丢失
	_ = a.SaveAuthConfig(id, &MCPAuthConfig{
		ClientID:         clientID,
		RedirectURI:      redirectURI,
		AuthorizationURL: as.AuthorizationEndpoint,
		TokenURL:         as.TokenEndpoint,
		RegisterURL:      as.RegistrationEndpoint,
		Scopes:           scope,
	})

	a.UpdateStatus(id, "testing", "等待浏览器完成授权...")
	return au.String(), nil
}

// handleOAuthCallback loopback 回调：校验 state → code 换 token → 加密落库
func (a *MCPServerApi) handleOAuthCallback(w http.ResponseWriter, r *http.Request, id uint, flow *oauthPendingFlow) {
	writeResult := func(ok bool, msg string) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		status := "failed"
		if ok {
			status = "success"
		}
		fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>go-stock MCP 授权</title></head>
<body style="font-family:system-ui;text-align:center;padding-top:60px">
<h2>%s</h2><p style="color:#666">%s</p><p style="color:#999">此页面可关闭，返回 go-stock 继续操作</p>
<script>setTimeout(()=>window.close(),3000)</script></body></html>`, map[bool]string{true: "✅ 授权成功", false: "❌ 授权失败"}[ok], msg)
		_ = status
	}

	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		writeResult(false, "授权服务器返回错误: "+e+" "+q.Get("error_description"))
		cleanupFlow(id, flow)
		a.UpdateStatus(id, "unauthorized", "授权被拒绝: "+e)
		return
	}
	if q.Get("state") != flow.State {
		writeResult(false, "state 校验失败（可能为过期或伪造的回调）")
		cleanupFlow(id, flow)
		return
	}
	code := q.Get("code")
	if code == "" {
		writeResult(false, "回调缺少 authorization code")
		cleanupFlow(id, flow)
		return
	}

	tok, err := exchangeOAuthCode(context.Background(), flow.TokenURL, flow.ClientID, code, flow.RedirectURI, flow.Verifier)
	if err != nil {
		writeResult(false, "Token 交换失败: "+err.Error())
		cleanupFlow(id, flow)
		a.UpdateStatus(id, "unauthorized", "Token 交换失败: "+err.Error())
		return
	}

	cfg, _ := a.LoadAuthConfig(id)
	if cfg == nil {
		cfg = &MCPAuthConfig{}
	}
	cfg.ClientID = flow.ClientID
	cfg.RedirectURI = flow.RedirectURI
	cfg.TokenURL = flow.TokenURL
	cfg.AccessToken = tok.AccessToken
	cfg.RefreshToken = tok.RefreshToken
	cfg.TokenType = tok.TokenType
	if tok.ExpiresIn > 0 {
		cfg.ExpiresAt = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second).Unix()
	} else {
		cfg.ExpiresAt = 0 // 未知有效期，交由 401 触发刷新
	}

	if err := a.SaveAuthConfig(id, cfg); err != nil {
		writeResult(false, "凭证保存失败: "+err.Error())
		cleanupFlow(id, flow)
		return
	}
	// 同步 TokenExpireAt 供前端展示
	expireAt := time.Time{}
	if cfg.ExpiresAt > 0 {
		expireAt = time.Unix(cfg.ExpiresAt, 0)
	}
	_ = dbDao().Model(&models.MCPServer{}).Where("id = ?", id).Update("token_expire_at", expireAt).Error

	writeResult(true, "已获得访问凭证，请在 go-stock 中测试连接")
	cleanupFlow(id, flow)
	a.UpdateStatus(id, "unauthorized", "授权成功，请点击「测试」验证连接")
}

func cleanupFlow(id uint, flow *oauthPendingFlow) {
	oauthFlowMu.Lock()
	if f, ok := oauthFlows[id]; ok && f == flow {
		delete(oauthFlows, id)
	}
	oauthFlowMu.Unlock()
	if flow.Server != nil {
		_ = flow.Server.Close()
	}
}

// exchangeOAuthCode authorization_code + PKCE 换 token
func exchangeOAuthCode(ctx context.Context, tokenURL, clientID, code, redirectURI, verifier string) (*oauthTokenResponse, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}
	return postTokenRequest(ctx, tokenURL, form)
}

// refreshOAuthToken refresh_token 换新 access_token
func refreshOAuthToken(ctx context.Context, tokenURL, clientID, refreshToken string) (*oauthTokenResponse, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
	}
	return postTokenRequest(ctx, tokenURL, form)
}

func postTokenRequest(ctx context.Context, tokenURL string, form url.Values) (*oauthTokenResponse, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var tok oauthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, fmt.Errorf("解析 token 响应失败: %w", err)
	}
	if tok.Error != "" {
		return nil, fmt.Errorf("token 端点错误: %s %s", tok.Error, tok.ErrorDescription)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("token 响应缺少 access_token (HTTP %d)", resp.StatusCode)
	}
	return &tok, nil
}

// ------------------------- 凭证解析与自动刷新 -------------------------

// resolveOAuthHeaders 解密凭证返回 Bearer 头。access token 临近过期（<10min）
// 或已过期时先用 refresh_token 刷新；刷新失败降级返回旧 token（由 401 状态兜底提示）。
func resolveOAuthHeaders(server *models.MCPServer) map[string]string {
	if server.AuthType != "oauth" || server.AuthConfig == "" {
		return nil
	}
	cfg, err := decryptAuthConfig(server.AuthConfig)
	if err != nil {
		logger.SugaredLogger.Warnf("MCP [%s] 凭证解密失败: %v", server.Name, err)
		return nil
	}
	if cfg.AccessToken == "" {
		return nil
	}

	needRefresh := cfg.ExpiresAt > 0 && time.Now().Unix() > cfg.ExpiresAt-600
	if needRefresh && cfg.RefreshToken != "" && cfg.TokenURL != "" {
		if tok, terr := refreshOAuthToken(context.Background(), cfg.TokenURL, cfg.ClientID, cfg.RefreshToken); terr == nil {
			cfg.AccessToken = tok.AccessToken
			if tok.RefreshToken != "" {
				cfg.RefreshToken = tok.RefreshToken
			}
			if tok.ExpiresIn > 0 {
				cfg.ExpiresAt = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second).Unix()
			}
			_ = dbDao().Model(&models.MCPServer{}).Where("id = ?", server.ID).
				Update("auth_config", mustEncrypt(cfg))
			logger.SugaredLogger.Infof("MCP [%s] OAuth token 已自动刷新", server.Name)
		} else {
			logger.SugaredLogger.Warnf("MCP [%s] OAuth token 刷新失败: %v", server.Name, terr)
		}
	}

	if strings.EqualFold(cfg.TokenType, "bearer") || cfg.TokenType == "" {
		return map[string]string{"Authorization": "Bearer " + cfg.AccessToken}
	}
	return map[string]string{"Authorization": cfg.TokenType + " " + cfg.AccessToken}
}

func mustEncrypt(cfg *MCPAuthConfig) string {
	sealed, err := encryptAuthConfig(cfg)
	if err != nil {
		return ""
	}
	return sealed
}

// ------------------------- 401/403 识别 -------------------------

// isUnauthorizedErr 判断 MCP 调用错误是否为鉴权失败（401/403），
// 用于区分「凭证问题需重新授权」与「服务器不可用」。
func isUnauthorizedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, kw := range []string{
		"401", "403", "status 401", "status 403",
		"unauthorized", "forbidden",
		"missing or malformed authorization",
	} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

// dbDao 统一 DB 访问入口（本文件多处使用，便于维护）
func dbDao() *gorm.DB { return db.Dao }
