//go:build zui

package zsettings

import (
	"github.com/torlangballe/zui/zcheckbox"
	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zimageview"
	"github.com/torlangballe/zui/zstyle"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zui/zwindow"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zlocale"
)

type SettingsView struct {
	zcontainer.StackView
}

var PopupViewFunc func(view, on zview.View)

func AddSettingsChecks(s *zcontainer.StackView, changedFunc func()) {
	addSettingsCheck(s, "Week Starts on Monday", zlocale.IsMondayFirstInWeek, changedFunc)
	addSettingsCheck(s, "Show Week Numbers", zlocale.IsShowWeekNumbersInCalendars, changedFunc)
	addSettingsCheck(s, "Use 24-hour Clock", zlocale.IsUse24HourClock, changedFunc)
	addSettingsCheck(s, "Show Month before Day", zlocale.IsShowMonthBeforeDay, changedFunc)
}

func addSettingsCheck(s *zcontainer.StackView, title string, option *zkeyvalue.Option[bool], changedFunc func()) *zcheckbox.CheckBox {
	check, label, stack := zcheckbox.NewWithLabel(false, title, "")
	label.SetColor(zgeo.ColorNewGray(0.2, 1))
	s.Add(stack, zgeo.CenterLeft)
	if option != nil {
		check.SetOn(option.Get())
		check.SetValueHandler("", func(edited bool) {
			option.Set(check.On(), true)
			if changedFunc != nil {
				changedFunc()
			}
		})
	}
	return check
}

func NewSettingsView() *SettingsView {
	v := &SettingsView{}
	v.Init(v, true, "settings")
	v.SetMarginS(zgeo.SizeBoth(10))
	v.SetBGColor(zgeo.ColorLightGray)
	AddSettingsChecks(&v.StackView, nil)
	check, label, stack := zcheckbox.NewWithLabel(zstyle.Dark, "Dark Mode", "zstyle.DarkMode")
	label.SetColor(zgeo.ColorNewGray(0.2, 1))
	v.Add(stack, zgeo.CenterLeft)
	check.SetValueHandler("", func(edited bool) {
		zstyle.Dark = check.On()
		zkeyvalue.DefaultStore.SetBool(zstyle.Dark, "zstyle.DarkMode", true)
		zwindow.GetMain().Reload()
	})
	return v
}

func NewSettingsIcon() *zimageview.ImageView {
	icon := zimageview.NewWithCachedPath("images/zcore/settings.png", zgeo.SizeD(18, 18))
	icon.MixColorForDarkMode = zgeo.ColorGray
	icon.SetPressedHandler("", 0, func() {
		popSettingsView(icon)
	})
	return icon
}

func popSettingsView(icon *zimageview.ImageView) {
	v := NewSettingsView()
	PopupViewFunc(v, icon)
}
