package web

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"trojan/core"
	"trojan/util"
	"trojan/web/controller"
)

var (
	identityKey    = "id"
	authMiddleware *jwt.GinJWTMiddleware
	err            error
	loginLimiter   = newRateLimiter(5, 15*time.Minute)
)

// Login auth用户验证结构体
type Login struct {
	Username string `form:"username" json:"username" binding:"required"`
	Password string `form:"password" json:"password" binding:"required"`
}

// rateLimiter 简单的基于来源IP的登录失败限速器
type rateLimiter struct {
	mu          sync.Mutex
	attempts    map[string]*attemptInfo
	maxFailures int
	lockWindow  time.Duration
}

type attemptInfo struct {
	failures int
	lockedAt time.Time
}

func newRateLimiter(maxFailures int, lockWindow time.Duration) *rateLimiter {
	return &rateLimiter{
		attempts:    make(map[string]*attemptInfo),
		maxFailures: maxFailures,
		lockWindow:  lockWindow,
	}
}

// locked 判断该IP当前是否处于锁定状态
func (r *rateLimiter) locked(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	info, ok := r.attempts[ip]
	if !ok {
		return false
	}
	if info.failures < r.maxFailures {
		return false
	}
	if time.Since(info.lockedAt) > r.lockWindow {
		// 锁定窗口已过, 重置
		delete(r.attempts, ip)
		return false
	}
	return true
}

// fail 记录一次失败
func (r *rateLimiter) fail(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	info, ok := r.attempts[ip]
	if !ok {
		info = &attemptInfo{}
		r.attempts[ip] = info
	}
	info.failures++
	if info.failures >= r.maxFailures {
		info.lockedAt = time.Now()
	}
}

// reset 登录成功后清除失败计数
func (r *rateLimiter) reset(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.attempts, ip)
}

func getSecretKey() string {
	sk, _ := core.GetValue("secretKey")
	// 旧版本使用 math/rand 生成 15 位弱密钥, 长度不足则重新生成强密钥
	if len(sk) < 32 {
		sk = util.RandString(48, util.ALL)
		core.SetValue("secretKey", sk)
	}
	return sk
}

// hashPassword 对(前端已哈希的)口令再做一次 bcrypt, 避免明文/可逆存储
func hashPassword(pass string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// setPassword 存储某个用户的 web 登录口令(bcrypt)
func setPassword(username, pass string) error {
	h, err := hashPassword(pass)
	if err != nil {
		return err
	}
	return core.SetValue(fmt.Sprintf("%s_pass", username), h)
}

// verifyPassword 校验口令; 兼容旧的明文/sha224 存量, 校验通过后自动迁移到 bcrypt
func verifyPassword(username, submitted string) bool {
	stored, e := core.GetValue(fmt.Sprintf("%s_pass", username))
	if e != nil || stored == "" {
		return false
	}
	if strings.HasPrefix(stored, "$2") {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(submitted)) == nil
	}
	// 旧存量: 常量时间比较, 通过后透明迁移为 bcrypt
	if subtle.ConstantTimeCompare([]byte(stored), []byte(submitted)) == 1 {
		if h, err := hashPassword(submitted); err == nil {
			core.SetValue(fmt.Sprintf("%s_pass", username), h)
		}
		return true
	}
	return false
}

func jwtInit(timeout int, secure bool) {
	authMiddleware, err = jwt.New(&jwt.GinJWTMiddleware{
		Realm:          "trojan-manager",
		Key:            []byte(getSecretKey()),
		Timeout:        time.Minute * time.Duration(timeout),
		MaxRefresh:     time.Minute * time.Duration(timeout),
		IdentityKey:    identityKey,
		SendCookie:     true,
		SecureCookie:   secure,
		CookieHTTPOnly: true,
		CookieSameSite: http.SameSiteStrictMode,
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if v, ok := data.(*Login); ok {
				return jwt.MapClaims{
					identityKey: v.Username,
				}
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			return &Login{
				Username: claims[identityKey].(string),
			}
		},
		Authenticator: func(c *gin.Context) (interface{}, error) {
			ip := c.ClientIP()
			if loginLimiter.locked(ip) {
				return nil, jwt.ErrFailedAuthentication
			}
			var loginVals Login
			if err := c.ShouldBind(&loginVals); err != nil {
				return "", jwt.ErrMissingLoginValues
			}
			userID := loginVals.Username
			pass := loginVals.Password

			authOK := false
			if userID == "admin" {
				authOK = verifyPassword("admin", pass)
			} else {
				mysql := core.GetMysql()
				user := mysql.GetUserByName(userID)
				if user != nil {
					authOK = subtle.ConstantTimeCompare([]byte(user.EncryptPass), []byte(pass)) == 1
				}
			}
			if authOK {
				loginLimiter.reset(ip)
				return &loginVals, nil
			}
			loginLimiter.fail(ip)
			return nil, jwt.ErrFailedAuthentication
		},
		Authorizator: func(data interface{}, c *gin.Context) bool {
			if _, ok := data.(*Login); ok {
				return true
			}
			return false
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			c.JSON(code, gin.H{
				"code":    code,
				"message": message,
			})
		},
		TokenLookup:   "header: Authorization, query: token, cookie: jwt",
		TokenHeadName: "Bearer",
		TimeFunc:      time.Now,
	})

	if err != nil {
		fmt.Println("JWT Error:" + err.Error())
	}
}

// registerHandler 仅在系统尚无管理员时可用(首次安装), 之后一律拒绝, 防止未授权重置管理员口令
func registerHandler(c *gin.Context) {
	responseBody := controller.ResponseBody{Msg: "success"}
	defer controller.TimeCost(time.Now(), &responseBody)
	// 首装守卫: 已存在管理员则拒绝
	if adminPass, _ := core.GetValue("admin_pass"); adminPass != "" {
		c.JSON(403, gin.H{"code": 403, "message": "管理员已存在, 禁止重复注册"})
		return
	}
	pass := c.PostForm("password")
	if pass == "" {
		c.JSON(400, gin.H{"code": 400, "message": "password不能为空"})
		return
	}
	// 未授权注册只允许创建 admin, 忽略传入的其它用户名
	if err := setPassword("admin", pass); err != nil {
		responseBody.Msg = err.Error()
	}
	c.JSON(200, responseBody)
}

// resetPassHandler 已鉴权的改密接口
func resetPassHandler(c *gin.Context) {
	responseBody := controller.ResponseBody{Msg: "success"}
	defer controller.TimeCost(time.Now(), &responseBody)
	username := c.DefaultPostForm("username", "admin")
	pass := c.PostForm("password")
	if pass == "" {
		c.JSON(400, gin.H{"code": 400, "message": "password不能为空"})
		return
	}
	if err := setPassword(username, pass); err != nil {
		responseBody.Msg = err.Error()
	}
	c.JSON(200, responseBody)
}

// RequestUsername 获取请求接口的用户名
func RequestUsername(c *gin.Context) string {
	claims := jwt.ExtractClaims(c)
	return claims[identityKey].(string)
}

// Auth 权限router
func Auth(r *gin.Engine, timeout int, secure bool) *jwt.GinJWTMiddleware {
	jwtInit(timeout, secure)

	newInstall := gin.H{"code": 201, "message": "No administrator account found inside the database", "data": nil}
	r.NoRoute(authMiddleware.MiddlewareFunc(), func(c *gin.Context) {
		c.JSON(404, gin.H{"code": 404, "message": "Page not found"})
	})
	r.GET("/auth/check", func(c *gin.Context) {
		result, _ := core.GetValue("admin_pass")
		if result == "" {
			c.JSON(201, newInstall)
		} else {
			title, err := core.GetValue("login_title")
			if err != nil {
				title = "trojan 管理平台"
			}
			c.JSON(200, gin.H{
				"code":    200,
				"message": "success",
				"data": map[string]string{
					"title": title,
				},
			})
		}
	})
	r.POST("/auth/login", authMiddleware.LoginHandler)
	r.POST("/auth/register", registerHandler)
	authO := r.Group("/auth")
	authO.Use(authMiddleware.MiddlewareFunc())
	{
		authO.GET("/loginUser", func(c *gin.Context) {
			result, _ := core.GetValue("admin_pass")
			if result == "" {
				c.JSON(201, newInstall)
			} else {
				c.JSON(200, gin.H{
					"code":    200,
					"message": "success",
					"data": map[string]string{
						"username": RequestUsername(c),
					},
				})
			}
		})
		authO.POST("/reset_pass", resetPassHandler)
		authO.POST("/logout", authMiddleware.LogoutHandler)
		authO.POST("/refresh_token", authMiddleware.RefreshHandler)
	}
	return authMiddleware
}
