package useragent_test

import (
	"testing"

	"github.com/mailgun/events"
	"github.com/mailgun/holster/useragent"
	. "gopkg.in/check.v1"
)

var UATests = []events.ClientInfo{
	/* Browsers */
	events.ClientInfo{
		ClientName: "Chrome",
		ClientOS:   "Linux",
		ClientType: useragent.ClientBrowser,
		DeviceType: useragent.DeviceDesktop,
		UserAgent:  "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.17 (KHTML, like Gecko) Chrome/24.0.1312.70 Safari/537.17",
	},
	events.ClientInfo{
		ClientName: "Safari",
		ClientOS:   "Mac OS X",
		ClientType: useragent.ClientBrowser,
		DeviceType: useragent.DeviceDesktop,
		UserAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_6) AppleWebKit/603.2.5 (KHTML, like Gecko) Version/10.1.1 Safari/603.2.5",
	},
	events.ClientInfo{
		ClientName: "Edge",
		ClientOS:   "Windows 10",
		ClientType: useragent.ClientBrowser,
		DeviceType: useragent.DeviceDesktop,
		UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.79 Safari/537.36 Edge/14.14393",
	},

	/* tablet */
	events.ClientInfo{
		ClientName: "Chrome",
		ClientOS:   "Android",
		ClientType: useragent.ClientBrowser,
		DeviceType: useragent.DeviceTablet,
		UserAgent:  "Mozilla/5.0 (Linux; Android 4.1.1; Nexus 7 Build/JRO03D) AppleWebKit/535.19 (KHTML, like Gecko) Chrome/18.0.1025.166  Safari/535.19",
	},
	events.ClientInfo{
		ClientName: "Mobile Safari UI/WKWebView",
		ClientOS:   "iOS",
		ClientType: useragent.ClientMobileBrowser,
		DeviceType: useragent.DeviceTablet,
		UserAgent:  "Mozilla/5.0 (iPad; CPU OS 10_3_3 like Mac OS X) AppleWebKit/603.3.8 (KHTML, like Gecko) Mobile/14G60",
	},

	/* mobile */
	events.ClientInfo{
		ClientName: "Firefox Mobile",
		ClientOS:   "Android",
		ClientType: useragent.ClientMobileBrowser,
		DeviceType: useragent.DeviceMobile,
		UserAgent:  "Mozilla/5.0 (Android 5.1.1; Mobile; rv:50.0) Gecko/50.0 Firefox/50.0",
	},
	events.ClientInfo{
		ClientName: "Mobile Safari",
		ClientOS:   "iOS",
		ClientType: useragent.ClientMobileBrowser,
		DeviceType: useragent.DeviceMobile,
		UserAgent:  "Mozilla/5.0 (iPhone; CPU iPhone OS 6_1_1 like Mac OS X) AppleWebKit/536.26 (KHTML, like Gecko) Version/6.0 Mobile/10B145 Safari/8536.25",
	},
	events.ClientInfo{
		ClientName: "Chrome Mobile",
		ClientOS:   "Android",
		ClientType: useragent.ClientMobileBrowser,
		DeviceType: useragent.DeviceMobile,
		UserAgent:  "Mozilla/5.0 (Linux; Android 4.4.4; SM-S820L Build/KTU84P) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/33.0.0.0 Mobile Safari/537.36",
	},

	/* Email clients */
	events.ClientInfo{
		ClientName: "Thunderbird",
		ClientOS:   "Linux",
		ClientType: useragent.ClientEmailClient,
		DeviceType: useragent.DeviceDesktop,
		UserAgent:  "Mozilla/5.0 (X11; Linux x86_64; rv:17.0) Gecko/20130106 Thunderbird/17.0.2",
	},
	events.ClientInfo{
		ClientName: "Apple Mail",
		ClientOS:   "Mac OS X",
		ClientType: useragent.ClientEmailClient,
		DeviceType: useragent.DeviceDesktop,
		UserAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/603.3.8 (KHTML, like Gecko)",
	},
	events.ClientInfo{
		ClientName: "Windows Live Mail",
		ClientOS:   "Windows 8",
		ClientType: useragent.ClientEmailClient,
		DeviceType: useragent.DeviceDesktop,
		UserAgent:  "Outlook-Express/7.0 (MSIE 7.0; Windows NT 6.2; WOW64; Trident/7.0; .NET4.0C; .NET4.0E; .NET CLR 2.0.50727; .NET CLR 3.0.30729; .NET CLR 3.5.30729; TmstmpExt)",
	},
	events.ClientInfo{
		ClientName: "Outlook",
		ClientOS:   "Windows 7",
		ClientType: useragent.ClientEmailClient,
		DeviceType: useragent.DeviceDesktop,
		UserAgent:  "Mozilla/4.0 (compatible; MSIE 7.0; Windows NT 6.1; WOW64; Trident/7.0; SLCC2; .NET CLR 2.0.50727; .NET CLR 3.5.30729; .NET CLR 3.0.30729; Media Center PC 6.0; InfoPath.2; .NET4.0C; .NET4.0E; InfoPath.3; MSOffice 12)",
	},
	events.ClientInfo{
		ClientName: "Lotus Notes",
		ClientOS:   "Windows",
		ClientType: useragent.ClientEmailClient,
		DeviceType: useragent.DeviceUnknown,
		UserAgent:  "Mozilla/4.0 (compatible; Lotus-Notes/6.0; Windows-NT)",
	},
	events.ClientInfo{
		ClientName: "Outlook",
		ClientOS:   "Windows Phone",
		ClientType: useragent.ClientEmailClient,
		DeviceType: useragent.DeviceMobile,
		UserAgent:  "Mozilla/5.0 (compatible; MSIE 10.0; Windows Phone 8.0; Trident/6.0; IEMobile/10.0; ARM; Touch; NOKIA; Lumia 520; ms-office; MSOffice 14)",
	},

	/* Bots */
	events.ClientInfo{
		ClientName: "Googlebot",
		ClientOS:   useragent.ClientUnknown,
		ClientType: useragent.ClientRobot,
		DeviceType: useragent.DeviceOther,
		UserAgent:  "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
	},
	events.ClientInfo{
		ClientName: "bingbot",
		ClientOS:   useragent.ClientUnknown,
		ClientType: useragent.ClientRobot,
		DeviceType: useragent.DeviceOther,
		UserAgent:  "Mozilla/5.0 (compatible; Bingbot/2.0; +http://www.bing.com/bingbot.htm)",
	},
	events.ClientInfo{
		ClientName: "DuckDuckBot",
		ClientOS:   useragent.ClientUnknown,
		ClientType: useragent.ClientRobot,
		DeviceType: useragent.DeviceOther,
		UserAgent:  "DuckDuckBot/1.0; (+http://duckduckgo.com/duckduckbot.html)",
	},
	events.ClientInfo{
		ClientName: "Baiduspider",
		ClientOS:   useragent.ClientUnknown,
		ClientType: useragent.ClientRobot,
		DeviceType: useragent.DeviceOther,
		UserAgent:  "Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)",
	},

	/* unknown */
	events.ClientInfo{
		ClientName: "Outlook",
		ClientOS:   useragent.ClientUnknown,
		ClientType: useragent.ClientEmailClient,
		DeviceType: useragent.DeviceUnknown,
		UserAgent:  "Microsoft Office/16.0 (Microsoft Outlook 16.0.8241; Pro)",
	},
	events.ClientInfo{
		ClientName: useragent.ClientUnknown,
		ClientOS:   useragent.ClientUnknown,
		ClientType: useragent.ClientUnknown,
		DeviceType: useragent.DeviceUnknown,
		UserAgent:  "lua-resty-http/0.10 (Lua) ngx_lua/10000",
	},
	events.ClientInfo{
		ClientName: useragent.ClientUnknown,
		ClientOS:   useragent.ClientUnknown,
		ClientType: useragent.ClientUnknown,
		DeviceType: useragent.DeviceUnknown,
		UserAgent:  "Crap",
	},
}

func Test(t *testing.T) {
	TestingT(t)
}

type UASuite struct{}

var _ = Suite(&UASuite{})

func (s *UASuite) SetUpSuite(c *C) {
	useragent.RegexesPath = "./assets/uap_regexes.yaml"
}

func (s *UASuite) TestParse(c *C) {
	for _, ua := range UATests {
		parsed, err := useragent.Parse(ua.UserAgent)
		c.Assert(err, IsNil)
		c.Assert(parsed, Equals, ua)
	}
}
