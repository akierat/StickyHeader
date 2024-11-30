package stickyheader

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"runtime"

	"github.com/hashicorp/golang-lru"
)

// Config the plugin configuration.
type Config struct {
	CacheSize  int    `json:"cacheSize,omitempty"`
	HeaderName string `json:"headerName,omitempty"`
	CookieName string `json:"cookieName,omitempty"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		CacheSize:  1000, // 默认缓存大小为 1000
		HeaderName: "user_id",
		CookieName: "whoami_session",
	}
}

type CookieManager struct {
	next   http.Handler
	Config *Config
	name   string
	cache  *lru.Cache
}

func (c *CookieManager) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	header := req.Header.Get(c.Config.HeaderName)
	if header == "" {
		c.next.ServeHTTP(rw, req)
		return
	}

	cookie, found := c.cache.Get(header)
	fmt.Printf("cache found: %v, header: %v, cookie: %v \n", found, header, cookie)
	if found {
		// 手动将新设置的 Cookie 添加到请求中
		req.AddCookie(&http.Cookie{
			Name:     c.Config.CookieName,
			Value:    cookie.(string),
			Path:     "/",
			HttpOnly: true,
		})
	}

	// 创建一个 ResponseRecorder 用于捕获后端服务的响应
	rec := &responseRecorder{
		ResponseWriter: rw,
		header:         http.Header{},
		body:           &bytes.Buffer{},
	}

	// 调用下一个 Handler
	c.next.ServeHTTP(rec, req)
	rw.WriteHeader(rec.statusCode)

	// 从捕获的响应中查找后端服务设置的 Cookie
	fmt.Printf("resp cookie: %v, len: %v \n", rec.cookies, len(rec.cookies))
	for _, cookie := range rec.cookies {
		if cookie.Name == c.Config.CookieName {
			fmt.Printf("update cookie, header: %s, cookie: %s\n", header, cookie.Value)
			c.cache.Add(header, cookie.Value)
		}
		http.SetCookie(rw, cookie)
	}

	// 写回响应体
	rw.Write(rec.body.Bytes())
}

// 定义一个 ResponseRecorder 用于捕获后端响应
type responseRecorder struct {
	http.ResponseWriter
	header     http.Header
	cookies    []*http.Cookie
	body       *bytes.Buffer
	statusCode int
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode

	// 解析 Set-Cookie 头
	if cookies, ok := r.header["Set-Cookie"]; ok {
		for _, c := range cookies {
			// 将 Set-Cookie 字符串解析成 http.Cookie 结构
			cookie, err := ParseSetCookie(c)
			if err != nil {
				fmt.Printf("failed to parse Set-Cookie, cookie: %v, err: %v", cookie, err)
				continue
			}
			r.cookies = append(r.cookies, cookie)
		}
	}

	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.body == nil {
		r.body = &bytes.Buffer{}
	}
	r.body.Write(b)
	return len(b), nil
}

func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	// 获取当前 Go 版本
	goVersion := runtime.Version()
	fmt.Printf("set up StickyHeader plugin, go version: %v, config: %v", goVersion, config)

	// 初始化 LRU 缓存
	cache, err := lru.New(config.CacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create LRU cache: %w", err)
	}

	return &CookieManager{
		Config: config,
		next:   next,
		name:   name,
		cache:  cache,
	}, nil
}
