package zscreen

import (
	"strconv"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xinerama"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
)

// "xxxgithub.com/kbinani/screenshot"

func GetAll() []Screen {
	var screens []Screen

	// defer func() {
	// 	e := recover()
	// 	if e != nil {
	// 		rect = image.Rectangle{}
	// 	}
	// }()

	c, err := xgb.NewConnDisplay(":0")
	if zlog.OnError(err, "NewConn") {
		return nil
	}
	defer c.Close()

	err = xinerama.Init(c)
	if zlog.OnError(err, "xinerama.Init") {
		return nil
	}
	reply, err := xinerama.QueryScreens(c).Reply()
	if zlog.OnError(err, "query-screens") {
		return nil
	}

	if reply.Number == 0 {
		zlog.Error("No screens!")
		return nil
	}

	primary := reply.ScreenInfo[0]
	x0 := float64(primary.XOrg)
	y0 := float64(primary.YOrg)

	for i := 0; i < int(reply.Number); i++ {
		var s Screen
		xscreen := reply.ScreenInfo[i]
		x := float64(xscreen.XOrg) - x0
		y := float64(xscreen.YOrg) - y0
		w := float64(xscreen.Width)
		h := float64(xscreen.Height)
		s.ID = strconv.Itoa(i)
		s.Rect = zgeo.RectFromXYWH(x, y, w, h)
		s.UsableRect = s.Rect // for now...
		s.Scale = 1
		s.SoftScale = 1
		s.IsMain = (i == 0)
		// zlog.Info("Screen:", s)
		screens = append(screens, s)
	}
	return screens
}
