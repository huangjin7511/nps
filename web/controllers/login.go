package controllers

import (
	"html/template"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/beego/beego"
	"github.com/beego/beego/cache"
	"github.com/beego/beego/utils/captcha"
	"github.com/djylb/nps/lib/common"
	"github.com/djylb/nps/lib/crypt"
	"github.com/djylb/nps/lib/file"
	"github.com/djylb/nps/lib/logs"
	"github.com/djylb/nps/server"
)

type LoginController struct {
	beego.Controller
}

const BanTime int64 = 5
const IpBanTime int64 = 180
const UserBanTime int64 = 3600
const MaxFailTimes int = 10
const MaxLoginBody = 1024

var loginRecord sync.Map
var cpt *captcha.Captcha
var powBits int
var secureMode bool
var forcePow bool

type record struct {
	hasLoginFailTimes int
	lastLoginTime     time.Time
}

func InitLogin() {
	secureMode = beego.AppConfig.DefaultBool("secure_mode", false)
	forcePow = beego.AppConfig.DefaultBool("force_pow", false)
	powBits = beego.AppConfig.DefaultInt("pow_bits", 20)
	// use beego cache system store the captcha data
	store := cache.NewMemoryCache()
	cpt = captcha.NewWithFilter(beego.AppConfig.String("web_base_url")+"/captcha/", store)
	cpt.ChallengeNums = 4
	cpt.StdWidth = 100
	cpt.StdHeight = 50
}

func (self *LoginController) Index() {
	// Try login implicitly, will succeed if it's configured as no-auth(empty username&password).
	webBaseUrl := beego.AppConfig.String("web_base_url")
	if self.doLogin("", "", "", false) {
		self.Redirect(webBaseUrl+"/index/index", 302)
		return
	}
	nonce := crypt.GetRandomString(16)
	self.SetSession("login_nonce", nonce)
	self.Data["login_nonce"] = nonce
	self.Data["pow_bits"] = powBits
	self.Data["totp_len"] = crypt.TotpLen
	self.Data["pow_enable"] = forcePow
	self.Data["public_key"], _ = crypt.GetRSAPublicKeyPEM()
	self.Data["login_delay"] = BanTime * 1000
	self.Data["web_base_url"] = webBaseUrl
	self.Data["head_custom_code"] = template.HTML(beego.AppConfig.String("head_custom_code"))
	self.Data["version"] = server.GetVersion()
	self.Data["year"] = server.GetCurrentYear()
	self.Data["register_allow"], _ = beego.AppConfig.Bool("allow_user_register")
	self.Data["captcha_open"], _ = beego.AppConfig.Bool("open_captcha")
	self.TplName = "login/index.html"
}

func (self *LoginController) Verify() {
	if self.Ctx.Request.ContentLength > MaxLoginBody {
		self.CustomAbort(413, "Payload too large")
		return
	}
	nonce := crypt.GetRandomString(16)
	stored := self.GetSession("login_nonce")
	self.SetSession("login_nonce", nonce)
	username := self.GetString("username")
	ip, _, _ := net.SplitHostPort(self.Ctx.Request.RemoteAddr)
	httpOnlyPass := beego.AppConfig.String("x_nps_http_only")
	if (beego.AppConfig.DefaultBool("allow_x_real_ip", false) && isTrustedProxy(ip)) ||
		(httpOnlyPass != "" && self.Ctx.Request.Header.Get("X-NPS-Http-Only") == httpOnlyPass) {
		if realIP := self.Ctx.Request.Header.Get("X-Real-IP"); realIP != "" {
			ip = realIP
		}
	}
	isIpBan := IsLoginBan(ip, IpBanTime)
	isUserBan := IsLoginBan(username, UserBanTime)
	totpCode := ""
	captchaOpen, _ := beego.AppConfig.Bool("open_captcha")
	cptVerify := true
	if captchaOpen {
		cptId := self.GetString(cpt.FieldIDName)
		cptCode := self.GetString(cpt.FieldCaptchaName)
		codeLen := len(cptCode)
		if codeLen >= crypt.TotpLen {
			totpCode = cptCode[codeLen-crypt.TotpLen:]
			cptCode = cptCode[:codeLen-crypt.TotpLen]
		}
		cptVerify = cpt.Verify(cptId, cptCode)
		if isIpBan || (!cptVerify && totpCode == "") || (!cptVerify && totpCode != "" && isUserBan) {
			logs.Warn("Captcha failed for user %s from %s", username, ip)
			IfLoginFail(ip, true)
			self.Data["json"] = map[string]interface{}{"status": 0, "msg": "the verification code is wrong, please get it again and try again", "nonce": nonce}
			self.SetSession("login_nonce", nonce)
			self.ServeJSON()
			return
		}
	}
	plRaw := self.GetString("password")
	if ((isUserBan && secureMode) || forcePow || (totpCode != "" && !cptVerify) || isIpBan) && powBits > 0 {
		powX := self.GetString("powx")
		bits, _ := self.GetInt("bits", 0)
		if bits != powBits || !common.ValidatePoW(powBits, plRaw, powX) {
			logs.Warn("PoW failed for user %s from %s", username, ip)
			IfLoginFail(ip, true)
			if !cptVerify {
				IfLoginFail(username, true)
			}
			self.Data["json"] = map[string]interface{}{"status": 0, "msg": "pow verification failed", "nonce": nonce, "bits": powBits}
			self.SetSession("login_nonce", nonce)
			self.ServeJSON()
			return
		}
	}
	pl, err := crypt.ParseLoginPayload(plRaw)
	if err != nil {
		logs.Warn("Decrypt error for user %s from %s: %v", username, ip, err)
		IfLoginFail(ip, true)
		if !cptVerify {
			IfLoginFail(username, true)
		}
		cert, _ := crypt.GetRSAPublicKeyPEM()
		self.Data["json"] = map[string]interface{}{"status": 0, "msg": "decrypt error", "nonce": nonce, "cert": cert}
		self.ServeJSON()
		return
	}
	if stored == nil || stored.(string) != pl.Nonce {
		logs.Warn("Invalid nonce for user %s from %s", username, ip)
		IfLoginFail(ip, true)
		if !cptVerify {
			IfLoginFail(username, true)
		}
		self.Data["json"] = map[string]interface{}{"status": 0, "msg": "invalid nonce", "nonce": nonce}
		self.ServeJSON()
		return
	}
	now := time.Now().UnixMilli()
	if pl.Timestamp < now-5*60*1000 || pl.Timestamp > now+60*1000 {
		logs.Warn("Timestamp expired for user %s from %s", username, ip)
		IfLoginFail(ip, true)
		if !cptVerify {
			IfLoginFail(username, true)
		}
		self.Data["json"] = map[string]interface{}{"status": 0, "msg": "timestamp expired", "nonce": nonce}
		self.ServeJSON()
		return
	}
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(20)))
	if self.doLogin(username, pl.Password, totpCode, true) {
		logs.Info("Login success for user %s from %s", username, ip)
		self.DelSession("login_nonce")
		self.Data["json"] = map[string]interface{}{"status": 1, "msg": "login success"}
	} else {
		logs.Warn("Login failed for user %s from %s", username, ip)
		IfLoginFail(username, true)
		self.Data["json"] = map[string]interface{}{"status": 0, "msg": "username or password incorrect", "nonce": nonce}
	}
	self.ServeJSON()
}

func (self *LoginController) doLogin(username, password, totp string, explicit bool) bool {
	clearIprecord()
	ip, _, _ := net.SplitHostPort(self.Ctx.Request.RemoteAddr)
	httpOnlyPass := beego.AppConfig.String("x_nps_http_only")
	if (beego.AppConfig.DefaultBool("allow_x_real_ip", false) && isTrustedProxy(ip)) ||
		(httpOnlyPass != "" && self.Ctx.Request.Header.Get("X-NPS-Http-Only") == httpOnlyPass) {
		if realIP := self.Ctx.Request.Header.Get("X-Real-IP"); realIP != "" {
			ip = realIP
		}
	}

	if explicit && IsLoginBan(ip, IpBanTime) {
		return false
	}

	var auth bool
	if adminAuth(username, password, totp) {
		self.SetSession("isAdmin", true)
		self.DelSession("clientId")
		self.DelSession("username")
		auth = true
		server.Bridge.Register.Store(common.GetIpByAddr(self.Ctx.Input.IP()), time.Now().Add(time.Hour*time.Duration(2)))
	}
	b, err := beego.AppConfig.Bool("allow_user_login")
	if err == nil && b && !auth && username != "" && password != "" {
		file.GetDb().JsonDb.Clients.Range(func(key, value interface{}) bool {
			v := value.(*file.Client)
			if !v.Status || v.NoDisplay {
				return true
			}
			if v.WebUserName == "" && v.WebPassword == "" {
				if v.Id <= 0 || username != "user" || v.VerifyKey != password {
					return true
				} else {
					auth = true
				}
			}
			if !auth && v.WebUserName == username {
				pwdInput := password
				ok := true
				if v.WebTotpSecret != "" {
					ok = false
					if totp != "" {
						ok, _ = crypt.ValidateTOTPCode(v.WebTotpSecret, totp)
					} else {
						pLen := len(password)
						if pLen >= crypt.TotpLen {
							pwdInput = password[:pLen-crypt.TotpLen]
							code := password[pLen-crypt.TotpLen:]
							ok, _ = crypt.ValidateTOTPCode(v.WebTotpSecret, code)
						}
					}
				} else if v.WebPassword == "" && v.VerifyKey == password {
					auth = true
				}
				if !auth && ok && v.WebPassword == pwdInput {
					auth = true
				}
			}
			if auth {
				self.SetSession("isAdmin", false)
				self.SetSession("clientId", v.Id)
				self.SetSession("username", v.WebUserName)
				return false
			}
			return true
		})
	}
	if auth {
		self.SetSession("auth", true)
		loginRecord.Delete(ip)
		return true
	}
	IfLoginFail(ip, explicit)
	return false
}

func IfLoginFail(key string, explicit bool) {
	if v, load := loginRecord.LoadOrStore(key, &record{hasLoginFailTimes: 1, lastLoginTime: time.Now()}); load && explicit {
		vv := v.(*record)
		vv.lastLoginTime = time.Now()
		vv.hasLoginFailTimes += 1
		loginRecord.Store(key, vv)
	}
}

func IsLoginBan(key string, ti int64) bool {
	if v, ok := loginRecord.Load(key); ok {
		vv := v.(*record)
		duration := time.Now().Unix() - vv.lastLoginTime.Unix()
		if duration < BanTime {
			logs.Warn("%s request rate too high, login blocked", key)
			return true
		}
		if duration >= ti {
			vv.hasLoginFailTimes = 0
		}
		if vv.hasLoginFailTimes >= MaxFailTimes {
			logs.Warn("%s has reached maximum failed attempts, login blocked", key)
			return true
		}
	}
	return false
}

func (self *LoginController) Register() {
	if self.Ctx.Request.Method == "GET" {
		nonce := crypt.GetRandomString(16)
		self.SetSession("login_nonce", nonce)
		self.Data["login_nonce"] = nonce
		self.Data["public_key"], _ = crypt.GetRSAPublicKeyPEM()
		self.Data["web_base_url"] = beego.AppConfig.String("web_base_url")
		self.Data["head_custom_code"] = template.HTML(beego.AppConfig.String("head_custom_code"))
		self.Data["version"] = server.GetVersion()
		self.Data["year"] = server.GetCurrentYear()
		self.Data["captcha_open"], _ = beego.AppConfig.Bool("open_captcha")
		self.TplName = "login/register.html"
	} else {
		if b, err := beego.AppConfig.Bool("allow_user_register"); err != nil || !b {
			self.Data["json"] = map[string]interface{}{"status": 0, "msg": "register is not allow"}
			self.ServeJSON()
			return
		}
		nonce := crypt.GetRandomString(16)
		stored := self.GetSession("login_nonce")
		self.SetSession("login_nonce", nonce)
		if self.GetString("username") == "" || self.GetString("password") == "" || self.GetString("username") == beego.AppConfig.String("web_username") {
			self.Data["json"] = map[string]interface{}{"status": 0, "msg": "please check your input", "nonce": nonce}
			self.ServeJSON()
			return
		}
		captchaOpen, _ := beego.AppConfig.Bool("open_captcha")
		if captchaOpen {
			if !cpt.VerifyReq(self.Ctx.Request) {
				self.Data["json"] = map[string]interface{}{"status": 0, "msg": "the verification code is wrong, please get it again and try again", "nonce": nonce}
				self.SetSession("login_nonce", nonce)
				self.ServeJSON()
				return
			}
		}
		pl, err := crypt.ParseLoginPayload(self.GetString("password"))
		if err != nil {
			cert, _ := crypt.GetRSAPublicKeyPEM()
			self.Data["json"] = map[string]interface{}{"status": 0, "msg": "decrypt error", "nonce": nonce, "cert": cert}
			self.ServeJSON()
			return
		}
		if stored == nil || stored.(string) != pl.Nonce {
			self.Data["json"] = map[string]interface{}{"status": 0, "msg": "invalid nonce", "nonce": nonce}
			self.ServeJSON()
			return
		}
		now := time.Now().UnixMilli()
		if pl.Timestamp < now-5*60*1000 || pl.Timestamp > now+60*1000 {
			self.Data["json"] = map[string]interface{}{"status": 0, "msg": "timestamp expired", "nonce": nonce}
			self.ServeJSON()
			return
		}
		t := &file.Client{
			Id:          int(file.GetDb().JsonDb.GetClientId()),
			Status:      true,
			Cnf:         &file.Config{},
			WebUserName: self.GetString("username"),
			WebPassword: pl.Password,
			Flow:        &file.Flow{},
		}
		if err := file.GetDb().NewClient(t); err != nil {
			self.Data["json"] = map[string]interface{}{"status": 0, "msg": err.Error(), "nonce": nonce}
		} else {
			self.DelSession("login_nonce")
			self.Data["json"] = map[string]interface{}{"status": 1, "msg": "register success"}
		}
		self.ServeJSON()
	}
}

func (self *LoginController) Out() {
	self.SetSession("auth", false)
	self.Redirect(beego.AppConfig.String("web_base_url")+"/login/index", 302)
}

func clearIprecord() {
	rand.Seed(time.Now().UnixNano())
	x := rand.Intn(100)
	if x == 1 {
		loginRecord.Range(func(key, value interface{}) bool {
			v := value.(*record)
			if time.Now().Unix()-v.lastLoginTime.Unix() >= UserBanTime {
				loginRecord.Delete(key)
			}
			return true
		})
	}
}

func adminAuth(username, password, totp string) bool {
	//logs.Error("login %s %s", username, password)
	expectedUser := beego.AppConfig.String("web_username")
	if username != expectedUser {
		//logs.Error("username is wrong")
		return false
	}
	totpSecret := beego.AppConfig.String("totp_secret")
	expectedPass := beego.AppConfig.String("web_password")
	if totpSecret != "" {
		ok := false
		if totp != "" {
			ok, _ = crypt.ValidateTOTPCode(totpSecret, totp)
		} else {
			pLen := len(password)
			if pLen < crypt.TotpLen {
				return false
			}
			code := password[pLen-crypt.TotpLen:]
			password = password[:pLen-crypt.TotpLen]
			ok, _ = crypt.ValidateTOTPCode(totpSecret, code)
		}
		if !ok {
			return false
		}
	}
	return password == expectedPass
}

func isTrustedProxy(ip string) bool {
	list := beego.AppConfig.DefaultString("trusted_proxy_ips", "127.0.0.1")
	for _, entry := range strings.Split(list, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		// if CIDR
		if strings.Contains(entry, "/") {
			if _, cidrNet, err := net.ParseCIDR(entry); err == nil {
				if cidrNet.Contains(net.ParseIP(ip)) {
					return true
				}
			}
			continue
		}

		// if "192.168.*.*"
		if strings.Contains(entry, "*") {
			pSegs := strings.Split(entry, ".")
			ipSegs := strings.Split(ip, ".")
			if len(pSegs) == 4 && len(ipSegs) == 4 {
				matched := true
				for i := 0; i < 4; i++ {
					if pSegs[i] == "*" {
						continue
					}
					if pSegs[i] != ipSegs[i] {
						matched = false
						break
					}
				}
				if matched {
					return true
				}
			}
			continue
		}

		if entry == ip {
			return true
		}
	}
	return false
}
