//go:build zui

package xrpc

import (
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztimer"
)

var (
	registeredResources []string
	gettingResources    zmap.LockMap[string, bool]
	pollGetters         zmap.LockMap[string, func()]
)

func PollForUpdatedResources(got func(resID string)) {
	for _, r := range registeredResources {
		got(r)
		f, got := pollGetters.Get(r)
		// zlog.Info("PollForUpdatedResources", r, got)
		if got {
			f()
		}
	}
	ztimer.RepeatForever(1, func() {
		var resIDs []string
		err := MainCaller().Call("XRPCResourceCalls.GetUpdatedResourcesAndSetSent", nil, &resIDs)
		if err != nil {
			zlog.Error("updateResources err:", err)
			return
		}
		// zlog.Info("PollForUpdatedResources", resIDs, registeredResources)
		for _, s := range resIDs {
			if !zstr.StringsContain(registeredResources, s) {
				continue
			}
			setting, _ := gettingResources.Get(s)
			if setting {
				continue
			}
			gettingResources.Set(s, true)
			f, has := pollGetters.Get(s)
			if has {
				f()
			} else {
				got(s)
			}
			gettingResources.Set(s, false)
		}
	})
}

func CallGetForUpdatedResources(resIDs []string) {
	for _, s := range resIDs {
		if !zstr.StringsContain(registeredResources, s) {
			continue
		}
		setting, _ := gettingResources.Get(s)
		if setting {
			continue
		}
		gettingResources.Set(s, true)
		f, has := pollGetters.Get(s)
		if has {
			f()
		}
		gettingResources.Set(s, false)
	}
}

func RegisterResources(resources ...string) {
	registeredResources = zstr.UnionStringSet(registeredResources, resources)
}

func RegisterPollGetter(resID string, get func()) {
	// zlog.Info("RegisterPollGetter", resID)
	RegisterResources(resID)
	pollGetters.Set(resID, get)
}

func (c Caller) CallToDownload(method, filename string, input any) error {
	var path string
	err := c.Call(method, input, &path)
	if err != nil {
		return err
	}
	surl := path
	zview.DownloadURI(surl, filename)
	return nil
}

// Dummy handleServerConnectionError needed to match one called for server
func (r *RPC) handleServerConnectionError(pipeID string, err error) {}
