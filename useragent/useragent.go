package useragent

import (
	"fmt"
	"strings"

	"github.com/avct/uasurfer"
	"github.com/mailgun/events"
	"github.com/ua-parser/uap-go/uaparser"
)

const (
	DeviceUnknown = "unknown"
	DeviceDesktop = "desktop"
	DeviceMobile  = "mobile"
	DeviceTablet  = "tablet"
	DeviceOther   = "other" // e.g. bots

	ClientUnknown       = "unknown"
	ClientBrowser       = "browser"
	ClientMobileBrowser = "mobile browser"
	ClientEmailClient   = "email client"
	ClientRobot         = "robot"
)

var (
	EmailClients = [...]string{"Windows Live Mail", "Outlook", "Apple Mail", "Thunderbird", "Lotus Notes", "Postbox", "Sparrow", "PocoMail"}
	RegexesPath  = "/var/mailgun/uap_regexes.yaml"
	uasParser    *uaparser.Parser
)

func Parse(uagent string) (events.ClientInfo, error) {
	var err error

	// Load the UAP regexes if not already done
	if uasParser == nil {
		if uasParser, err = uaparser.New(RegexesPath); err != nil {
			return events.ClientInfo{}, fmt.Errorf("Failed to init UA parser: %s", err)
		}
	}

	surferParsedAgent := uasurfer.Parse(uagent)
	uapParsedAgent := uasParser.Parse(uagent)

	deviceType := getDeviceType(surferParsedAgent.DeviceType)
	clientName := uapParsedAgent.UserAgent.Family
	if clientName == "Other" {
		clientName = ClientUnknown
	}
	clientOs := uapParsedAgent.Os.Family
	if clientOs == "Other" {
		clientOs = ClientUnknown
	}
	clientType := getClientType(uapParsedAgent, surferParsedAgent)
	if clientType == ClientRobot {
		deviceType = DeviceOther
	}

	return events.ClientInfo{
		UserAgent:  uagent,
		DeviceType: deviceType,
		ClientName: clientName,
		ClientOS:   clientOs,
		ClientType: clientType,
	}, nil
}

func getDeviceType(deviceType uasurfer.DeviceType) string {
	// map mailgun types to uasurfer types
	switch deviceType {
	case uasurfer.DeviceUnknown:
		return DeviceUnknown
	case uasurfer.DeviceComputer:
		return DeviceDesktop
	case uasurfer.DeviceTablet:
		return DeviceTablet
	case uasurfer.DevicePhone:
		return DeviceMobile
	case uasurfer.DeviceConsole, uasurfer.DeviceWearable, uasurfer.DeviceTV:
		return DeviceOther
	default:
		return DeviceUnknown
	}
}

func getClientType(uapUA *uaparser.Client, surferUA *uasurfer.UserAgent) string {
	clientName := uapUA.UserAgent.Family
	switch {
	case isEmailClient(clientName):
		return ClientEmailClient
	case strings.Contains(clientName, "Mobi") || surferUA.DeviceType == uasurfer.DevicePhone:
		return ClientMobileBrowser
	case surferUA.OS.Platform == uasurfer.PlatformBot || surferUA.OS.Name == uasurfer.OSBot:
		return ClientRobot
	case surferUA.Browser.Name >= uasurfer.BrowserBot && surferUA.Browser.Name <= uasurfer.BrowserYahooBot:
		return ClientRobot
	case surferUA.DeviceType == uasurfer.DeviceComputer || surferUA.DeviceType == uasurfer.DeviceTablet:
		return ClientBrowser
	default:
		return ClientUnknown
	}
}

func isEmailClient(name string) bool {
	for _, a := range EmailClients {
		if a == name {
			return true
		}
	}
	return false
}
