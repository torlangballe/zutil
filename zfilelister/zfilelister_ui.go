//go:build zui

package zfilelister

import (
	"path"
	"strings"

	"github.com/torlangballe/zui/zalert"
	"github.com/torlangballe/zui/zcheckbox"
	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zgridlist"
	"github.com/torlangballe/zui/zimageview"
	"github.com/torlangballe/zui/zlabel"
	"github.com/torlangballe/zui/zpresent"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrpc"
	"github.com/torlangballe/zutil/zslice"
	"github.com/torlangballe/zutil/zstr"
)

// type Options struct {
// 	ViewOnly          bool
// 	ChooseFolders     bool
// 	ExtensionsAllowed []string //
// 	PickedPaths       []string // ends in / if folders
// 	StoreName         string
// 	ToToken           string
// }

type FileListerView struct {
	zcontainer.StackView
	DirOptions

	DirFunc     func(dirOpts DirOptions, got func(paths []string, err error))
	GetImageURL func(path string) string
	PickedPaths []string // ends in / if folders

	back       *zlabel.Label
	title      *zlabel.Label
	errorLabel *zlabel.Label
	grid       *zgridlist.GridListView
}

const (
	iconID  = "icon"
	titleID = "title"
	checkID = "check"
)

func makeLabel(text string) *zlabel.Label {
	label := zlabel.New(text)
	return label
}

func NewFileListerView(opts DirOptions) *FileListerView {
	// zlog.Info("NewFileListerView:", basePath, storeName)
	v := &FileListerView{}
	v.SetMarginS(zgeo.SizeBoth(5))
	v.Init(v, true, opts.StoreName+".FileLister")

	v.DirOptions = opts
	bar := zcontainer.StackViewHor("bar")
	v.Add(bar, zgeo.TopLeft|zgeo.HorExpand)

	back := zimageview.New(nil, true, "images/triangle-left-gray.png", zgeo.SizeD(16, 20))
	bar.Add(back, zgeo.TopLeft)

	v.title = makeLabel("")
	bar.Add(v.title, zgeo.TopLeft|zgeo.HorExpand)

	v.grid = zgridlist.NewView(opts.StoreName)
	v.grid.MakeFullSize = true
	v.grid.SetMinSize(zgeo.SizeD(340, 400))
	v.grid.MinRowsForFullSize = 5
	v.grid.MaxRowsForFullSize = 20
	v.grid.CellCountFunc = func() int {
		return len(v.DirOptions.PickedPaths)
	}
	v.grid.CreateCellFunc = v.createRow
	v.grid.UpdateCellFunc = v.updateRow
	v.Add(v.grid, zgeo.TopLeft|zgeo.Expand)

	v.errorLabel = zlabel.New("")
	v.errorLabel.SetMaxWidth(300)
	v.errorLabel.Columns = 2
	v.errorLabel.SetColor(zgeo.ColorRed)
	v.Add(v.errorLabel, zgeo.TopLeft|zgeo.HorExpand)

	return v
}

func (v *FileListerView) ReadyToShow(beforeWindow bool) {
	if beforeWindow {
		v.update()
	}
}

func (v *FileListerView) pathOfID(id string) string {
	index := v.grid.IndexOfID(id)
	return v.DirOptions.PickedPaths[index]
}

func (v *FileListerView) updateRow(grid *zgridlist.GridListView, id string) {
	row := v.grid.CellView(id).(*zcontainer.StackView)
	path := v.pathOfID(id)

	iurl := v.GetImageURL(path)
	f, _ := row.FindViewWithName(iconID, false)
	iv := f.(*zimageview.ImageView)
	iv.SetImage(nil, iurl, nil)

	f, _ = row.FindViewWithName(titleID, false)
	label := f.(*zlabel.Label)
	label.SetText(path)

	f, _ = row.FindViewWithName(checkID, false)
	check := f.(*zcheckbox.CheckBox)
	on := zbool.False
	for _, p := range v.PickedPaths {
		if p == path {
			on = zbool.True
			break
		}
		if strings.HasPrefix(p, path) {
			on = zbool.Unknown
			break
		}
	}
	check.SetValue(on)
}

func (v *FileListerView) createRow(grid *zgridlist.GridListView, id string) zview.View {
	s := zcontainer.StackViewHor(id)
	s.SetMarginS(zgeo.SizeD(4, 3))
	s.SetSpacing(6)

	check := zcheckbox.New(zbool.False)
	check.SetObjectName(checkID)
	check.SetValueHandler(func() {
		on := check.On()
		path := v.pathOfID(id)
		zslice.RemoveIf(v.PickedPaths, func(i int) bool {
			return strings.HasPrefix(v.PickedPaths[i], path)
		})
		if on {
			v.PickedPaths = append(v.PickedPaths, path) // we can add it, as it will be removed above
		} else {
			v.PickedPaths = zstr.RemovedFromSlice(v.PickedPaths, path)
		}
	})
	s.Add(check, zgeo.CenterLeft)

	icon := zimageview.New(nil, true, "", v.IconSize)
	icon.SetObjectName(iconID)
	s.Add(icon, zgeo.CenterLeft)

	title := makeLabel("")
	title.SetObjectName(titleID)
	s.Add(title, zgeo.CenterLeft|zgeo.HorExpand)

	return s
}

func (v *FileListerView) update() {
	v.DirFunc(v.DirOptions, func(paths []string, err error) {
		if zlog.OnError(err, "DirFunc", v.DirOptions.PathStub) {
			return
		}
		v.DirOptions.PickedPaths = paths
		v.grid.LayoutCells(true)
	})
}

// func MakeGetURL(storeName, urlPrefix, urlStub, toToken string) string {
// 	if !zhttp.StringStartsWithHTTPX(urlPrefix) {
// 		urlPrefix = zfile.JoinPathParts(zapp.URLStub(), zrest.AppURLPrefix)
// 	}
// 	if urlStub == "" {
// 		urlStub = "zfilelister-files/" + storeName
// 	}
// 	basePrefix := zfile.JoinPathParts(urlPrefix, urlStub)
// 	if toToken != "" {
// 		basePrefix += "?token=" + toToken
// 	}
// 	return basePrefix
// }

func NewRemoteFileListerView(urlPrefix, urlStub string, opts DirOptions) *FileListerView {
	flister := NewFileListerView(opts)
	flister.DirFunc = func(dirOpts DirOptions, got func(paths []string, err error)) {
		go func() {
			var paths []string
			err := zrpc.MainClient.Call("FileServerCalls.GetDirectory", dirOpts, &paths)
			if err != nil {
				got(nil, err)
				return
			}
			got(paths, nil)
		}()
	}
	// zlog.Info("NewRemoteFileLister: DirFunc:", flister.DirFunc != nil)
	flister.GetImageURL = func(spath string) string {
		ext := path.Ext(spath)
		hash := zstr.HashTo64Hex(spath)
		surl := zfile.JoinPathParts(urlPrefix, "caches/filelister-icons/"+opts.StoreName, hash+ext)
		zlog.Info("RemoteFileLister:Image", spath, surl)
		return surl
	}
	return flister
}

func (v *FileListerView) Present(title string, got func(pickedPaths []string)) {
	att := zpresent.ModalDialogAttributes
	zalert.PresentOKCanceledView(v, title, att, nil, func(ok bool) bool {
		if ok {
			got(v.PickedPaths)
		}
		return true
	})
}
