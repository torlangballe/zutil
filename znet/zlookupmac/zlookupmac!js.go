//go:build !js

package zlookupmac

import (
	"os"
	"time"

	"github.com/torlangballe/zutil/zcache"
	"github.com/torlangballe/zutil/zhttp"
)

type Manufacturer struct {
	Company     string `json:"company"`
	Address     string `json:"address"`
	CountryCode string `json:"country"`
}

var (
	Cache        = zcache.NewExpiringMap[string, Manufacturer](3600 * 24)
	LastDone     time.Time
	FileCacheSet bool
	ApiKey       string
)

var apple = Manufacturer{Company: "Apple, Inc.", Address: "1 Infinite Loop, Cupertino CA 95014, US", CountryCode: "US"}

var hardcodedRandomMACs = map[string]Manufacturer{
	"96:be:ef": apple,
}

func ForceCacheSave() {
	Cache.FlushToStorage()
}

func LookupManufacturer(mac string, fileCache bool) (Manufacturer, bool, error) {
	mac = mac[:8]
	if fileCache && !FileCacheSet {
		FileCacheSet = true
		Cache.SetStorage("mac-lookup-cache")
	}
	m, got := Cache.Get(mac)
	if got {
		return m, true, nil
	}
	since := time.Since(LastDone)
	LastDone = time.Now()
	left := time.Millisecond*22 - since // we rate-limit to every 20 millisecs (50 a sec)
	if left > 0 {
		time.Sleep(left)
	}
	var err error
	var result struct {
		Manufacturer
		Found bool `json:"found"`
	}
	params := zhttp.MakeParameters()
	// params.PrintBody = true
	surl := "https://api.maclookup.app/v2/macs/" + mac

	if ApiKey == "" {
		ApiKey = os.Getenv("MAC_LOOKUP_APP_APIKEY")
	}
	surl += "?apiKey=" + ApiKey
	// surl, err = zhttp.GetRedirectedURL(surl)
	// if err != nil {
	// 	return m, err
	// }
	_, err = zhttp.Get(surl, params, &result)
	if err != nil {
		return m, false, err
	}
	// zlog.Info("LookupMACAddressToManufacurer found:", result.Found, surl)
	if !result.Found {
		m, got = hardcodedRandomMACs[mac]
		if !got {
			return m, false, nil
		}
	} else {
		m = result.Manufacturer
	}
	Cache.Set(mac, m)
	return m, true, nil
}
